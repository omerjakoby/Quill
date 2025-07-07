package domain

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/html"
	"io"
	"log"
	"quill/cmd/main/constants"
	"regexp"
	"strings"
	"time"
)

// EmailService defines the interface for message-related operations

// MockEmailService implements the EmailService interface with mock data

type MockEmailService struct{}

/**
// Send implements a mock message sending operation
func (m *MockEmailService) Send(ctx context.Context, req SendEmailRequest) (SendEmailResult, error) {
	log.Println("Mock: Sending message to", req.To)
	return SendEmailResult{
		MessageID:   "mock-msg-123",
		ThreadID:    "mock-thread-456",
		DeliveredTo: req.To,

	}, nil
}

 Fetch implements a mock message fetching operation
func (m *MockEmailService) Fetch(ctx context.Context, req FetchEmailRequest) (FetchEmailResult, error) {
	log.Println("Mock: Fetching messages with mode", req.Mode)
	return FetchEmailResult{
		Total:    0,
		Limit:    10,
		Offset:   0,
		Messages: []Message{},
	}, nil
}
**/

// MongoEmailService implements the EmailService interface with MongoDB storage
type MongoEmailService struct {
	db *mongo.Database
}

// NewMongoEmailService creates a new MongoDB-backed EmailService
func NewMongoEmailService(db *mongo.Database) *MongoEmailService {
	return &MongoEmailService{
		db: db,
	}
}

// mailboxEntryOptions represents the options for a mailbox entry
type mailboxEntryOptions struct {
	Read     bool   `bson:"read"`
	Category string `bson:"category,omitempty"`
	Starred  bool   `bson:"starred,omitempty"` // indicates if the message is starred
}

// mailboxEntry represents a reference to a message in a user's mailbox
type mailboxEntry struct {
	UserID     string              `bson:"quillMail"` // quillmail address of the user
	MessageID  string              `bson:"messageId"`
	ThreadID   string              `bson:"threadId"`
	Folder     string              `bson:"folder"`
	ReceivedAt time.Time           `bson:"receivedAt"`
	Options    mailboxEntryOptions `bson:"options"`
}

// Send stores a message in MongoDB and adds entries to each recipient's mailbox
func (m *MongoEmailService) SendEmail(ctx context.Context, req SendEmailRequest) (SendEmailResult, error) {
	// Validate the request
	if validateQuillMailFormat(req.From) {
		if extractDomain(req.From) == constants.DOMAIN_NAME {
			return m.SendInternal(ctx, req)
		}
		return m.SendExternal(ctx, req)
	}
	return SendEmailResult{}, errorString("invalid sender address format")

}

func (m *MongoEmailService) SendInternal(ctx context.Context, req SendEmailRequest) (SendEmailResult, error) {
	authUserID, ok := UserIDFromContext(ctx)
	if !ok {
		return SendEmailResult{}, ErrUserNotAuthenticated
	}

	authUserQuillMail, err := m.getUserQuillMail(ctx, authUserID)
	if err != nil {
		return SendEmailResult{}, err
	}

	if req.From != authUserQuillMail {
		return SendEmailResult{}, errorString("sender address does not match authenticated user")
	}

	// Validate recipient address format
	for _, addr := range append(append(req.To, req.CC...), req.BCC...) {
		if !validateQuillMailFormat(addr) {
			return SendEmailResult{}, errorString(fmt.Sprintf("invalid recipient address format: %s", addr))
		}
	}

	messageID, err := getOrValidateMessageID(req.MessageID)
	if err != nil {
		return SendEmailResult{}, err
	}

	threadID, err := getOrValidateThreadID(req.ThreadID)
	if err != nil {
		return SendEmailResult{}, err
	}

	now := time.Now().UTC()

	messageDoc := bson.M{
		"messageId":   messageID,
		"fromID":      authUserID,
		"fromMail":    req.From,
		"to":          req.To,
		"cc":          req.CC,
		"bcc":         req.BCC,
		"subject":     req.Subject,
		"body":        req.Body,
		"attachments": req.Attachments,
		"sentAt":      now,
		"options": bson.M{
			"expiresInSeconds": req.Options.ExpiresInSeconds,
			"oneTime":          req.Options.OneTime,
			"threadID":         threadID,
		},
	}

	if _, err := m.db.Collection("messages").InsertOne(ctx, messageDoc); err != nil {
		log.Printf("Failed to insert message: %v", err)
		return SendEmailResult{}, err
	}

	entries := []interface{}{
		mailboxEntry{
			UserID:     authUserQuillMail,
			MessageID:  messageID,
			ThreadID:   threadID,
			Folder:     "sent",
			ReceivedAt: now,
			Options: mailboxEntryOptions{
				Read: true,
			},
		},
	}

	category, err := m.GetCategory(ctx, req)
	if err != nil {
		log.Printf("Failed to get category for message: %v", err)
		return SendEmailResult{}, err
	}

	allRecipients := append(append(req.To, req.CC...), req.BCC...)
	internal, external, newEntries := splitRecipientsAndBuildEntries(allRecipients, messageID, threadID, category, now)
	entries = append(entries, newEntries...)

	if len(entries) > 0 {
		if _, err := m.db.Collection("mailboxes").InsertMany(ctx, entries); err != nil {
			log.Printf("Failed to insert mailbox entries: %v", err)
			// consider rollback of the message?
		}
	}

	return SendEmailResult{
		MessageID:   messageID,
		ThreadID:    threadID,
		DeliveredTo: internal,
		QueuedFor:   external,
	}, nil
}

func (m *MongoEmailService) SendExternal(ctx context.Context, req SendEmailRequest) (SendEmailResult, error) {
	// Validate and extract threadID and messageID
	threadID, err := validateThreadID(req.ThreadID)
	if err != nil {
		return SendEmailResult{}, err
	}
	messageID, err := validateMessageID(req.MessageID)
	if err != nil {
		return SendEmailResult{}, err
	}

	for _, addr := range append(append(req.To, req.CC...), req.BCC...) {
		if !validateQuillMailFormat(addr) {
			return SendEmailResult{}, errorString(fmt.Sprintf("invalid recipient address format: %s", addr))
		}
	}

	// Check for existing message
	exists, err := m.messageExists(ctx, messageID)
	if err != nil {
		return SendEmailResult{}, err
	}
	if exists {
		return SendEmailResult{}, errorString("message with this ID already exists")
	}

	// Prepare message document
	now := time.Now().UTC()
	messageDoc := bson.M{
		"messageId":   messageID,
		"fromMail":    req.From,
		"to":          req.To,
		"cc":          req.CC,
		"subject":     req.Subject,
		"body":        req.Body,
		"attachments": req.Attachments,
		"sentAt":      now,
		"options": bson.M{
			"expiresInSeconds": req.Options.ExpiresInSeconds,
			"oneTime":          req.Options.OneTime,
			"threadID":         threadID,
		},
	}

	// Insert message into messages collection
	_, err = m.db.Collection("messages").InsertOne(ctx, messageDoc)
	if err != nil {
		log.Printf("Failed to insert message: %v", err)
		return SendEmailResult{}, err
	}

	category, err := m.GetCategory(ctx, req)
	if err != nil {
		log.Printf("Failed to get category for message: %v", err)
		return SendEmailResult{}, err
	}

	// Create mailbox entries for all internal recipients
	myRecipients := getInternalRecipients(req, constants.DOMAIN_NAME)
	mailboxEntries := createMailboxEntries(myRecipients, messageID, threadID, category, now)
	if len(mailboxEntries) > 0 {
		_, err = m.db.Collection("mailboxes").InsertMany(ctx, mailboxEntries)
		if err != nil {
			log.Printf("Failed to insert mailbox entries: %v", err)
			// Consider handling this error (perhaps delete the message?)
		}
	}

	return SendEmailResult{
		MessageID: messageID,
		ThreadID:  threadID,
	}, nil
}

// Helper to validate threadID
func validateThreadID(threadIDPtr *string) (string, error) {
	if threadIDPtr == nil || *threadIDPtr == "" || !isUUID(*threadIDPtr) {
		return "", errorString("did not provide thread ID")
	}
	return *threadIDPtr, nil
}

// Helper to validate messageID
func validateMessageID(messageID string) (string, error) {
	if messageID == "" {
		return "", errorString("did not provide message ID")
	}
	return messageID, nil
}

// Helper to check if a message exists
func (m *MongoEmailService) messageExists(ctx context.Context, messageID string) (bool, error) {
	singleResult := m.db.Collection("messages").FindOne(ctx, bson.M{"messageId": messageID})
	if err := singleResult.Err(); err != nil {
		if err != mongo.ErrNoDocuments {
			log.Printf("failed to check if message exists: %v", err)
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// Helper to get internal recipients
func getInternalRecipients(req SendEmailRequest, domain string) []string {
	var recipients []string
	for _, addr := range req.To {
		if extractDomain(addr) == domain {
			recipients = append(recipients, addr)
		}
	}
	for _, addr := range req.CC {
		if extractDomain(addr) == domain {
			recipients = append(recipients, addr)
		}
	}
	for _, addr := range req.BCC {
		if extractDomain(addr) == domain {
			recipients = append(recipients, addr)
		}
	}
	return recipients
}

// Helper to create mailbox entries
func createMailboxEntries(recipients []string, messageID, threadID string, category string, now time.Time) []interface{} {
	var entries []interface{}
	for _, recipient := range recipients {
		entries = append(entries, mailboxEntry{
			UserID:     recipient,
			MessageID:  messageID,
			ThreadID:   threadID,
			Folder:     "inbox",
			ReceivedAt: now,
			Options: mailboxEntryOptions{
				Read:     false,
				Category: category,
			},
		})
	}
	return entries
}

// Fetch retrieves messages based on the provided request
func (m *MongoEmailService) FetchEmail(ctx context.Context, req FetchEmailRequest) (FetchEmailResult, error) {
	// ... (Existing FetchEmail logic remains the same for initial mode checks)
	if (req.Mode == FetchModeThread && req.ThreadID == nil) || (req.Mode == FetchModeOverview && req.Folder == nil) {
		return FetchEmailResult{}, errorString("missing required parameters for fetch mode")
	} else if req.Mode != FetchModeThread && req.Mode != FetchModeOverview {
		return FetchEmailResult{}, errorString("invalid fetch mode")
	} else if req.Mode == FetchModeOverview {
		// THIS IS THE MODIFIED CALL
		return m.FetchOverview(ctx, req)
	} else if req.Mode == FetchModeThread {
		// FetchThread remains unchanged, as per our earlier discussion.
		return m.FetchThread(ctx, req)
	}
	return FetchEmailResult{}, errorString("unsupported fetch mode")
}

func (m *MongoEmailService) FetchOverview(ctx context.Context, req FetchEmailRequest) (FetchEmailResult, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return FetchEmailResult{}, ErrUserNotAuthenticated
	}

	quillmail, err := m.getUserQuillMail(ctx, userID)
	if err != nil {
		return FetchEmailResult{}, err
	}

	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
	}
	offset := 0
	if req.Offset != nil {
		offset = *req.Offset
	}

	// New: Build base mailbox filter including folder, is_read, is_starred
	baseFilter := buildBaseMailboxFilter(req, userID, quillmail)

	// New: Count unique threads with all filters applied
	total, err := m.countUniqueThreadsWithFilters(ctx, baseFilter, req.Filters)
	if err != nil {
		log.Printf("Failed to count unique threads with filters: %v", err)
		return FetchEmailResult{}, err
	}

	// New: Fetch mailbox entries AND joined message details with all filters
	results, err := m.fetchMailboxEntriesWithFilters(ctx, baseFilter, req.Filters, offset, limit)
	if err != nil {
		log.Printf("Failed to fetch mailbox entries with filters: %v", err)
		return FetchEmailResult{}, err
	}

	if len(results) == 0 {
		return FetchEmailResult{
			TotalThreads:    int(total),
			Limit:           limit,
			Offset:          offset,
			ThreadOverviews: []ThreadOverview{},
		}, nil
	}

	// New: Build ThreadOverviews from the raw BSON results of the aggregation
	threadOverviews := m.buildThreadOverviewsFromAggregation(ctx, results, userID, quillmail)

	return FetchEmailResult{
		TotalThreads:    int(total),
		Limit:           limit,
		Offset:          offset,
		ThreadOverviews: threadOverviews,
	}, nil
}

func (m *MongoEmailService) FetchThread(ctx context.Context, req FetchEmailRequest) (FetchEmailResult, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return FetchEmailResult{}, ErrUserNotAuthenticated
	}

	if req.ThreadID == nil {
		return FetchEmailResult{}, errorString("thread ID is required for thread mode")
	}

	quillmail, err := m.getUserQuillMail(ctx, userID)
	if err != nil {
		return FetchEmailResult{}, err
	}

	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
	}
	offset := 0
	if req.Offset != nil {
		offset = *req.Offset
	}

	// Check if the user has access to the thread
	if err := m.checkThreadAccess(ctx, req, userID, quillmail); err != nil {
		return FetchEmailResult{}, err
	}

	// Get all messages in the thread
	rawMessages, total, err := m.fetchThreadMessages(ctx, *req.ThreadID, offset, limit)
	if err != nil {
		return FetchEmailResult{}, err
	}

	if len(rawMessages) == 0 {
		return FetchEmailResult{
			TotalMessages: int(total),
			Limit:         limit,
			Offset:        offset,
			Messages:      []Message{},
		}, nil
	}

	messageIDs := extractMessageIDsFromRaw(rawMessages)
	readStatusMap, err := m.fetchReadStatusMap(ctx, quillmail, messageIDs)
	if err != nil {
		return FetchEmailResult{}, err
	}

	messages := mapRawMessagesToDomain(rawMessages, readStatusMap)

	return FetchEmailResult{
		TotalMessages: int(total),
		Limit:         limit,
		Offset:        offset,
		Messages:      messages,
	}, nil
}

// Helper: Check if user has access to the thread
func (m *MongoEmailService) checkThreadAccess(ctx context.Context, req FetchEmailRequest, userID, quillmail string) error {
	mailboxFilter := buildBaseMailboxFilter(req, userID, quillmail)
	count, err := m.db.Collection("mailboxes").CountDocuments(ctx, mailboxFilter)
	if err != nil {
		return err
	}
	if count == 0 {
		// Also check with the quillmail address
		mailboxFilter = bson.M{
			"quillMail": quillmail,
			"threadID":  *req.ThreadID,
		}
		count, err = m.db.Collection("mailboxes").CountDocuments(ctx, mailboxFilter)
		if err != nil {
			return err
		}
		if count == 0 {
			return errorString("thread not found or access denied")
		}
	}
	return nil
}

// Helper: Fetch messages in the thread with pagination
func (m *MongoEmailService) fetchThreadMessages(ctx context.Context, threadID string, offset, limit int) ([]bson.M, int64, error) {
	messageFilter := bson.M{
		"options.threadID": threadID,
	}
	total, err := m.db.Collection("messages").CountDocuments(ctx, messageFilter)
	if err != nil {
		return nil, 0, err
	}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "sentAt", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	messageCursor, err := m.db.Collection("messages").Find(ctx, messageFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = messageCursor.Close(ctx)
	}()
	var rawMessages []bson.M
	if err = messageCursor.All(ctx, &rawMessages); err != nil {
		return nil, 0, err
	}
	return rawMessages, total, nil
}

// Helper: Extract message IDs from raw messages
func extractMessageIDsFromRaw(rawMessages []bson.M) []string {
	var messageIDs []string
	for _, msg := range rawMessages {
		if msgID, ok := msg["messageId"].(string); ok {
			messageIDs = append(messageIDs, msgID)
		}
	}
	return messageIDs
}

// Helper: Fetch read status map for messages
func (m *MongoEmailService) fetchReadStatusMap(ctx context.Context, quillmail string, messageIDs []string) (map[string]bool, error) {
	readStatusFilter := bson.M{
		"quillMail": quillmail,
		"messageId": bson.M{"$in": messageIDs},
	}
	readStatusCursor, err := m.db.Collection("mailboxes").Find(ctx, readStatusFilter)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = readStatusCursor.Close(ctx)
	}()
	readStatusMap := make(map[string]bool)
	var mailboxEntries []mailboxEntry
	if err = readStatusCursor.All(ctx, &mailboxEntries); err != nil {
		return nil, err
	}
	for _, entry := range mailboxEntries {
		readStatusMap[entry.MessageID] = entry.Options.Read
	}
	return readStatusMap, nil
}

// Helper: Map raw messages to domain messages with read status
func mapRawMessagesToDomain(rawMessages []bson.M, readStatusMap map[string]bool) []Message {
	var messages []Message
	for _, rawMsg := range rawMessages {
		msgID, _ := rawMsg["messageId"].(string)
		read := readStatusMap[msgID] // false if not found in map
		message := convertBsonToMessage(rawMsg, read)
		messages = append(messages, message)
	}
	return messages
}

// getUserQuillMail retrieves the user's quillmail address from the users collection
func (m *MongoEmailService) getUserQuillMail(ctx context.Context, userID string) (string, error) {
	collection := m.db.Collection("users")
	filter := bson.M{"_id": userID}
	var result struct {
		UserQuillMail string `bson:"userQuillMail"`
	}
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", err
		}
		return "", fmt.Errorf("error retrieving userQuillMail: %w", err)
	}
	return result.UserQuillMail, nil
}

// buildBaseMailboxFilter builds the initial filter for mailbox queries,
// including user, folder, and mailbox-specific flags (read, starred).
func buildBaseMailboxFilter(req FetchEmailRequest, userID, quillmail string) bson.M {
	filter := bson.M{
		"quillMail": quillmail,
	}

	// Add folder filter
	if req.Folder != nil {
		filter["folder"] = *req.Folder
	} else {
		filter["folder"] = "inbox" // default if not provided
	}

	// Add mailbox-level flag filters (IsRead, IsStarred)
	if req.Filters != nil && req.Filters.Flags != nil {
		if req.Filters.Flags.IsRead != nil {
			filter["options.read"] = *req.Filters.Flags.IsRead
		}
		if req.Filters.Flags.IsStarred != nil {
			filter["options.starred"] = *req.Filters.Flags.IsStarred
		}
	}

	return filter
}

func (m *MongoEmailService) GetCategory(ctx context.Context, req SendEmailRequest) (string, error) {
	var htmlContent string
	if req.Body.HTML == "" {
		if req.Body.Text == "" {
			return "", errorString("no content provided for category extraction")
		}
		htmlContent = req.Body.Text
	} else {
		htmlContent = req.Body.HTML
	}

	if htmlContent == "" {
		return "", errorString("content is empty")
	}
	htmlContent, err := ExtractTextStream(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to extract text from HTML: %w", err)
	}
	categories := ClassifyEmail(req.Subject, htmlContent)
	return categories.BestCategory, nil
}

// ExtractTextStream reads HTML from r and returns all visible text,
// ignore S3776
func ExtractTextStream(r io.Reader) (string, error) {
	z := html.NewTokenizer(r)
	var buf bytes.Buffer
	skip := false

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				// done
				goto CLEANUP
			}
			return "", z.Err()

		case html.StartTagToken:
			t := z.Token()
			if t.Data == "script" || t.Data == "style" {
				skip = true
			}

		case html.EndTagToken:
			t := z.Token()
			if t.Data == "script" || t.Data == "style" {
				skip = false
			}

		case html.TextToken:
			if skip {
				continue
			}
			txt := strings.TrimSpace(string(z.Text()))
			if txt != "" {
				buf.WriteString(txt)
				buf.WriteByte(' ')
			}

		default:
			// Ignore other token types, do nothing

		}
	}

CLEANUP:
	// Normalize whitespace: collapse runs of space to one, trim ends.
	fields := strings.Fields(buf.String())
	return strings.Join(fields, " "), nil
}

// fetchMailboxEntries retrieves mailbox entries with sorting, skip, and limit
// Only returns the first message of each unique thread
// fetchMailboxEntriesWithFilters retrieves mailbox entries with sorting, skip, and limit,
// applying all specified filters and joining with messages collection.
// It returns raw BSON documents representing the combined mailboxEntry and messageDetails.
func (m *MongoEmailService) fetchMailboxEntriesWithFilters(ctx context.Context, baseFilter bson.M, filters *FetchEmailFilters, offset, limit int) ([]bson.M, error) {
	pipeline := []bson.M{}

	// Stage 1: Match mailbox entries (applies user, folder, is_read, is_starred)
	pipeline = append(pipeline, bson.M{"$match": baseFilter})

	// Stage 2: Sort by receivedAt descending (within mailbox entries for initial ordering)
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"receivedAt": -1}})

	// Stage 3: Lookup messages with nested filters for message-specific criteria
	lookupPipeline := []bson.M{}

	// Apply message-level filters (keywords, exact_phrase, from, to, attachments, date_range)
	messageFilters := buildMessageFilters(filters)
	if len(messageFilters) > 0 {
		lookupPipeline = append(lookupPipeline, bson.M{"$match": messageFilters})
	}

	// Sort by sentAt descending to ensure we pick the latest *matching* message
	lookupPipeline = append(lookupPipeline, bson.M{"$sort": bson.M{"sentAt": -1}})
	lookupPipeline = append(lookupPipeline, bson.M{"$limit": 1}) // Only need the latest matching message

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "messages",       // The collection to join with
			"localField":   "messageId",      // Field from the input documents (mailboxes)
			"foreignField": "messageId",      // Field from the 'messages' documents
			"as":           "messageDetails", // Name of the new array field to add to the input documents
			"pipeline":     lookupPipeline,   // The nested pipeline for filtering joined documents
		},
	})

	// Stage 4: Filter out mailbox entries where no matching message was found by the lookup pipeline.
	// This removes threads whose latest message (or any message if not sorted) didn't satisfy filters.
	pipeline = append(pipeline, bson.M{"$match": bson.M{"messageDetails": bson.M{"$ne": []interface{}{}}}})

	// Stage 5: Unwind the messageDetails array. Since we limited to 1 in lookup, this flattens it.
	pipeline = append(pipeline, bson.M{"$unwind": "$messageDetails"})

	// Stage 6: Group by threadId to ensure we get only one document per unique thread.
	// "$first" picks the latest `doc` after all previous sorting and filtering.
	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": "$threadId",                // Group by the thread ID
			"doc": bson.M{"$first": "$$ROOT"}, // Take the first document (which will be the latest qualifying one)
		},
	})

	// Stage 7: Replace root with the grouped document to flatten the structure again.
	pipeline = append(pipeline, bson.M{"$replaceRoot": bson.M{"newRoot": "$doc"}})

	// Stage 8: Final sort on the combined document's messageDetails.sentAt for chronological thread order.
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"messageDetails.sentAt": -1}})

	// Stage 9: Apply pagination (skip and limit)
	pipeline = append(pipeline, bson.M{"$skip": offset})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	cursor, err := m.db.Collection("mailboxes").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			log.Printf("Error closing cursor in fetchMailboxEntriesWithFilters: %v", cerr)
		}
	}()

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// extractMessageIDs extracts message IDs from mailbox entries
func extractMessageIDs(entries []mailboxEntry) []string {
	var messageIDs []string
	for _, entry := range entries {
		messageIDs = append(messageIDs, entry.MessageID)
	}
	return messageIDs
}

// fetchMessagesByIDs fetches messages and maps them to domain Message objects in the order of entries
//func (m *MongoEmailService) fetchMessagesByIDs(ctx context.Context, messageIDs []string, entries []mailboxEntry) ([]ThreadOverview, error) {
//	messageMap, err := m.getMessageMapByIDs(ctx, messageIDs)
//	if err != nil {
//		return nil, err
//	}
//
//	userID, ok := UserIDFromContext(ctx)
//	if !ok {
//		return nil, ErrUserNotAuthenticated
//	}
//	quillmail, err := m.getUserQuillMail(ctx, userID)
//	if err != nil {
//		return nil, err
//	}
//
//	threadIDs := m.collectThreadIDsFromEntries(entries, messageMap)
//	totalCounts, err := m.batchCountMessagesInThreads(ctx, threadIDs)
//	if err != nil {
//		log.Printf("Error batch counting total messages: %v", err)
//		totalCounts = make(map[string]int64)
//	}
//
//	unreadCounts, err := m.batchCountUnreadMessagesInThreads(ctx, quillmail, threadIDs)
//	if err != nil {
//		log.Printf("Error batch counting unread messages: %v", err)
//		unreadCounts = make(map[string]int64)
//	}
//
//	messages := m.buildThreadOverviews(entries, messageMap, totalCounts, unreadCounts)
//	return messages, nil
//}

// Helper: get message map by IDs
func (m *MongoEmailService) getMessageMapByIDs(ctx context.Context, messageIDs []string) (map[string]bson.M, error) {
	messageFilter := bson.M{"messageId": bson.M{"$in": messageIDs}}
	messageCursor, err := m.db.Collection("messages").Find(ctx, messageFilter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := messageCursor.Close(ctx); err != nil {
			log.Printf("Error closing message cursor: %v", err)
		}
	}()
	messageMap := make(map[string]bson.M)
	var rawMessages []bson.M
	if err = messageCursor.All(ctx, &rawMessages); err != nil {
		return nil, err
	}
	for _, msg := range rawMessages {
		if msgID, ok := msg["messageId"].(string); ok {
			messageMap[msgID] = msg
		}
	}
	return messageMap, nil
}

// Helper: collect unique thread IDs from entries and messageMap
func (m *MongoEmailService) collectThreadIDsFromEntries(entries []mailboxEntry, messageMap map[string]bson.M) []string {
	threadIDSet := make(map[string]bool)
	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			threadID := getThreadIDFromBson(rawMsg)
			threadIDSet[threadID] = true
		}
	}
	var threadIDs []string
	for threadID := range threadIDSet {
		threadIDs = append(threadIDs, threadID)
	}
	return threadIDs
}

// Helper: build ThreadOverview slice
func (m *MongoEmailService) buildThreadOverviews(entries []mailboxEntry, messageMap map[string]bson.M, totalCounts, unreadCounts map[string]int64) []ThreadOverview {
	var messages []ThreadOverview
	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			threadID := getThreadIDFromBson(rawMsg)
			totalCount := totalCounts[threadID]
			if totalCount == 0 {
				totalCount = 1
			}
			unreadCount := unreadCounts[threadID]
			message := convertBsonToThreadOverview(rawMsg, entry.Options.Read, entry.Options.Starred)
			message.Count = int(totalCount)
			message.UnreadCount = int(unreadCount)
			messages = append(messages, message)
		}
	}
	return messages
}

// Helper function to convert BSON to Message domain object
func convertBsonToMessage(bsonMsg bson.M, read bool) Message {
	msg := Message{
		ID:          getStringFromBson(bsonMsg, "messageId"),
		ThreadID:    getThreadIDFromBson(bsonMsg),
		From:        getStringFromBson(bsonMsg, "fromMail"),
		To:          getStringArrayFromBson(bsonMsg, "to"),
		CC:          getStringArrayFromBson(bsonMsg, "cc"),
		BCC:         getStringArrayFromBson(bsonMsg, "bcc"),
		Subject:     getStringFromBson(bsonMsg, "subject"),
		Body:        getEmailBodyFromBson(bsonMsg),
		Attachments: getAttachmentsFromBson(bsonMsg),
		Timestamp:   getTimeFromBson(bsonMsg, "sentAt"),
		Flags: EmailFlags{
			IsRead: read,
		},
	}

	return msg
}

// Helper function to convert BSON to Message domain object
func convertBsonToThreadOverview(bsonMsg bson.M, read bool, stared bool) ThreadOverview {

	msg := ThreadOverview{
		ThreadID: getThreadIDFromBson(bsonMsg),
		LatestMessage: MessageSummary{
			ID:       getStringFromBson(bsonMsg, "messageId"),
			ThreadID: getThreadIDFromBson(bsonMsg),
			From:     getStringFromBson(bsonMsg, "fromMail"),
			Subject:  getStringFromBson(bsonMsg, "subject"),
			Snippet: func() string {
				txt := getTextBodyFromBson(bsonMsg)
				if len(txt) <= 100 {
					return txt
				}
				return txt[:100] + "..."
			}(),
			Timestamp: getTimeFromBson(bsonMsg, "sentAt"),
			Flags: EmailFlags{
				IsRead: read, HasAttachments: hasAttachmentsFromBson(bsonMsg), IsStarred: stared,
			},
		},
	}
	return msg
}

// hasAttachmentsFromBson checks if the given bson.M message has non-empty attachments.
func hasAttachmentsFromBson(bsonMsg bson.M) bool {
	attachments, ok := bsonMsg["attachments"]
	if !ok || attachments == nil {
		return false
	}
	switch arr := attachments.(type) {
	case []interface{}:
		return len(arr) > 0
	case primitive.A:
		return len(arr) > 0
	default:
		return false
	}
}

// Helper function to safely extract EmailBody from BSON
func getEmailBodyFromBson(bsonMsg bson.M) EmailBody {
	if bodyData, ok := bsonMsg["body"].(bson.M); ok {
		return EmailBody{
			Text: getStringFromBson(bodyData, "text"),
			HTML: getStringFromBson(bodyData, "html"),
		}
	}
	// Return empty EmailBody if type assertion fails
	return EmailBody{}
}

// Helper function to safely extract EmailBody from BSON
func getTextBodyFromBson(bsonMsg bson.M) string {
	if bodyData, ok := bsonMsg["body"].(bson.M); ok {
		return getStringFromBson(bodyData, "text")

	}
	return ""
}

func getAttachmentsFromBson(bsonMsg bson.M) []Attachment {
	var attachments []Attachment
	if attachs, ok := bsonMsg["attachments"].(primitive.A); ok {
		for _, a := range attachs {
			if attach, ok := a.(bson.M); ok {
				attachments = append(attachments, Attachment{
					Filename: getStringFromBson(attach, "filename"),
					Mimetype: getStringFromBson(attach, "mimetype"),
					Link:     getStringFromBson(attach, "link"),
				})
			}
		}
	}
	return attachments
}

func getStringFromBson(bsonMsg bson.M, key string) string {
	if val, ok := bsonMsg[key].(string); ok {
		return val
	}
	return ""
}

func getThreadIDFromBson(bsonMsg bson.M) string {
	if opts, ok := bsonMsg["options"].(bson.M); ok {
		if threadID, ok := opts["threadID"].(string); ok {
			return threadID
		}
	}
	return ""
}

func getStringArrayFromBson(bsonMsg bson.M, key string) []string {
	var arr []string
	if pArr, ok := bsonMsg[key].(primitive.A); ok {
		for _, t := range pArr {
			if str, ok := t.(string); ok {
				arr = append(arr, str)
			}
		}
	}
	return arr
}

func getTimeFromBson(bsonMsg bson.M, key string) time.Time {
	if sentAtDT, ok := bsonMsg[key].(primitive.DateTime); ok {
		return sentAtDT.Time()
	}
	return time.Time{}
}

// Deprecated: This function is kept for reference, but not used in the current implementation
/*func getBodyFromBson(bsonMsg bson.M) Body {
	var body Body
	bodyMap, ok := bsonMsg["body"].(bson.M)
	if !ok {
		return body
	}
	contentArr, ok := bodyMap["content"].(primitive.A)
	if !ok {
		return body
	}
	for _, c := range contentArr {
		contentItem := parseContentItem(c)
		if contentItem != nil {
			body.Content = append(body.Content, *contentItem)
		}
	}
	return body
}
*/

// Helper to parse a content item from BSON (deprecated, kept for reference)
/*
func parseContentItem(c interface{}) *Content {
	contentMap, ok := c.(bson.M)
	if !ok {
		return nil
	}
	var contentItem Content
	if t, ok := contentMap["type"].(string); ok {
		contentItem.Type = ContentType(t)
	}
	if v, ok := contentMap["value"].(string); ok {
		contentItem.Value = v
	}
	return &contentItem
}

*/

// ErrUserNotAuthenticated is returned when a user ID cannot be extracted from context
var ErrUserNotAuthenticated = error(errorString("user not authenticated"))

type errorString string

func (e errorString) Error() string {
	return string(e)
}

func extractDomain(input string) string {
	lastTildeIndex := strings.LastIndex(input, "~")
	if lastTildeIndex != -1 {
		return input[lastTildeIndex+1:]
	}
	return ""
}

func isUUID(input string) bool {
	_, err := uuid.Parse(input)
	return err == nil
}

// Helper to get or validate messageID
func getOrValidateMessageID(messageID string) (string, error) {
	if messageID != "" {
		if !isUUID(messageID) {
			return "", errorString("invalid message ID: must be a UUID")
		}
		return messageID, nil
	}
	return uuid.New().String(), nil
}

// Helper to get or validate threadID
func getOrValidateThreadID(threadIDPtr *string) (string, error) {
	if threadIDPtr != nil && *threadIDPtr != "" {
		if !isUUID(*threadIDPtr) {
			return "", errorString("invalid thread ID: must be a UUID")
		}
		return *threadIDPtr, nil
	}
	return uuid.New().String(), nil
}

// Helper to split recipients and build mailbox entries
func splitRecipientsAndBuildEntries(recipients []string, messageID, threadID string, category string, now time.Time) (internal []string, external []string, entries []interface{}) {
	for _, addr := range recipients {
		if strings.HasSuffix(addr, constants.DOMAIN_NAME) {
			internal = append(internal, addr)
			entries = append(entries, mailboxEntry{
				UserID:     addr,
				MessageID:  messageID,
				ThreadID:   threadID,
				Folder:     "inbox",
				ReceivedAt: now,
				Options: mailboxEntryOptions{
					Read:     false,
					Category: category,
				},
			})
		} else {
			external = append(external, addr)
		}
	}
	return
}

// validateQuillMailFormat checks if the input matches the format (username)~(mail.com)
func validateQuillMailFormat(input string) bool {
	parts := strings.Split(input, "~")
	if len(parts) != 2 {
		return false
	}
	username := parts[0]
	domain := parts[1]
	return username != "" && domain != "" && strings.Contains(domain, ".")
}

func (m *MongoEmailService) UpdateEmail(ctx context.Context, req UpdateEmailRequest) (UpdateEmailResult, error) {
	userid, ok := UserIDFromContext(ctx)
	if !ok {
		return UpdateEmailResult{}, ErrUserNotAuthenticated
	}
	quillMail, err := m.getUserQuillMail(ctx, userid)
	if err != nil {
		return UpdateEmailResult{}, err
	}

	results := []UpdateEmailResultItem{}
	for _, msgId := range req.MessageIDs {
		if !isUUID(msgId) {
			results = append(results, UpdateEmailResultItem{
				MessageID: msgId,
				Status:    "ERROR",
				Error: &EmailError{
					Code:    "INVALID_MESSAGE_ID",
					Message: "Invalid message ID format",
				},
			})
			continue
		}

		err = m.handleUpdateEmailFlags(ctx, quillMail, msgId, req)
		if err != nil {
			results = append(results, UpdateEmailResultItem{
				MessageID: msgId,
				Status:    "ERROR",
				Error: &EmailError{
					Code:    "UPDATE_FAILED",
					Message: err.Error(),
				},
			})
		} else {
			results = append(results, UpdateEmailResultItem{
				MessageID: msgId,
				Status:    "OK",
				Error:     nil,
			})
		}
	}

	return UpdateEmailResult{Results: results}, nil
}

// handleUpdateEmailFlags reduces complexity by handling all update/delete logic for a single message
func (m *MongoEmailService) handleUpdateEmailFlags(ctx context.Context, quillMail, msgId string, req UpdateEmailRequest) error {
	if req.Flags.IsDeleted != nil && *req.Flags.IsDeleted {
		_, err := m.db.Collection("mailboxes").DeleteOne(ctx, bson.M{"messageId": msgId, "quillMail": quillMail})
		if err != nil {
			log.Printf("Failed to delete mailbox entries for message %s: %v", msgId, err)
			return err
		}
		return nil
	}
	if req.Flags.IsRead != nil && *req.Flags.IsRead {
		if err := m.updateMailboxField(ctx, quillMail, msgId, "options.read", true); err != nil {
			return err
		}
	}
	if req.Flags.IsStarred != nil {
		if err := m.updateMailboxField(ctx, quillMail, msgId, "options.starred", *req.Flags.IsStarred); err != nil {
			return err
		}
	}
	if req.Category != nil {
		if err := m.updateMailboxField(ctx, quillMail, msgId, "options.category", *req.Category); err != nil {
			return err
		}
	}
	if req.Folder != nil {
		if err := m.updateMailboxField(ctx, quillMail, msgId, "folder", *req.Folder); err != nil {
			return err
		}
	}
	return nil
}

// updateMailboxField is a helper to update a single field in the mailbox document
func (m *MongoEmailService) updateMailboxField(ctx context.Context, quillMail, msgId, field string, value interface{}) error {
	_, err := m.db.Collection("mailboxes").UpdateOne(ctx,
		bson.M{"messageId": msgId, "quillMail": quillMail},
		bson.M{"$set": bson.M{field: value}})
	if err != nil {
		log.Printf("Failed to update %s for message %s: %v", field, msgId, err)
		return err
	}
	return nil
}

// countUniqueThreadsWithFilters counts the number of unique threads matching the given filters.
func (m *MongoEmailService) countUniqueThreadsWithFilters(ctx context.Context, baseFilter bson.M, filters *FetchEmailFilters) (int64, error) {
	pipeline := []bson.M{}

	// Stage 1: Match mailbox entries (user, folder, is_read, is_starred)
	pipeline = append(pipeline, bson.M{"$match": baseFilter})

	// Stage 2: Lookup messages with nested filters
	lookupPipeline := []bson.M{}

	messageFilters := buildMessageFilters(filters)
	if len(messageFilters) > 0 {
		lookupPipeline = append(lookupPipeline, bson.M{"$match": messageFilters})
	}
	lookupPipeline = append(lookupPipeline, bson.M{"$limit": 1}) // Only need one matching message to confirm thread's relevance

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "messages",
			"localField":   "messageId",
			"foreignField": "messageId",
			"as":           "messageDetails",
			"pipeline":     lookupPipeline,
		},
	})

	// Stage 3: Filter out entries where no matching message was found by the lookup
	pipeline = append(pipeline, bson.M{"$match": bson.M{"messageDetails": bson.M{"$ne": []interface{}{}}}})

	// Stage 4: Group by threadId to get unique threads that passed all filters
	pipeline = append(pipeline, bson.M{"$group": bson.M{"_id": "$threadId"}})

	// Stage 5: Count the number of unique threads
	pipeline = append(pipeline, bson.M{"$count": "totalThreads"})

	cursor, err := m.db.Collection("mailboxes").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := cursor.Close(ctx); cerr != nil {
			log.Printf("Error closing cursor in countUniqueThreadsWithFilters: %v", cerr)
		}
	}()

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	switch count := result[0]["totalThreads"].(type) {
	case int32:
		return int64(count), nil
	case int64:
		return count, nil
	case float64:
		return int64(count), nil
	default:
		return 0, fmt.Errorf("unexpected count type in countUniqueThreadsWithFilters: %T", count)
	}
}

// buildMessageFilters constructs a BSON filter for the 'messages' collection
// based on the provided FetchEmailFilters.
func buildMessageFilters(filters *FetchEmailFilters) bson.M {
	if filters == nil {
		return bson.M{} // No filters provided, return empty BSON map
	}

	var andConditions []bson.M

	// Search filters (keywords, exact phrase, from, to)
	if filters.Search != nil {
		// Keywords: match any keyword in subject or body.text (case-insensitive)
		if len(filters.Search.Keywords) > 0 {
			var keywordOrClauses []bson.M
			for _, keyword := range filters.Search.Keywords {
				// Using regex for contains with 'i' for case-insensitivity
				escapedKeyword := regexp.QuoteMeta(keyword)
				keywordOrClauses = append(keywordOrClauses,
					bson.M{"subject": bson.M{"$regex": escapedKeyword, "$options": "i"}},
					bson.M{"body.text": bson.M{"$regex": escapedKeyword, "$options": "i"}},
				)
			}
			if len(keywordOrClauses) > 0 {
				andConditions = append(andConditions, bson.M{"$or": keywordOrClauses})
			}
		}

		// Exact Phrase: match exact phrase in subject or body.text (case-insensitive, with word boundaries)
		if filters.Search.ExactPhrase != "" {
			// Escape special regex characters in the phrase
			escapedPhrase := regexp.QuoteMeta(filters.Search.ExactPhrase)
			regexPattern := "\\b" + escapedPhrase + "\\b"
			andConditions = append(andConditions, bson.M{"$or": []bson.M{
				{"subject": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"body.text": bson.M{"$regex": regexPattern, "$options": "i"}},
			}})
		}

		// From addresses: match sender email
		if len(filters.Search.From) > 0 {
			andConditions = append(andConditions, bson.M{"fromMail": bson.M{"$in": filters.Search.From}})
		}

		// To addresses: match any recipient in to, cc, or bcc arrays
		if len(filters.Search.To) > 0 {
			var toOrClauses []bson.M
			for _, recipient := range filters.Search.To {
				toOrClauses = append(toOrClauses,
					bson.M{"to": recipient},
					bson.M{"cc": recipient},
					bson.M{"bcc": recipient},
				)
			}
			if len(toOrClauses) > 0 {
				andConditions = append(andConditions, bson.M{"$or": toOrClauses})
			}
		}
	}

	// Flag filters (specific to messages collection: HasAttachments)
	if filters.Flags != nil && filters.Flags.HasAttachments != nil {
		if *filters.Flags.HasAttachments {
			// Check if 'attachments' field exists, is not null, and has elements
			andConditions = append(andConditions, bson.M{
				"attachments": bson.M{"$exists": true, "$ne": nil, "$not": bson.M{"$size": 0}},
			})
		} else {
			// Check if 'attachments' field doesn't exist, is null, or is an empty array
			andConditions = append(andConditions, bson.M{"$or": []bson.M{
				{"attachments": bson.M{"$exists": false}},
				{"attachments": nil},
				{"attachments": bson.M{"$size": 0}},
			}})
		}
	}

	// Date range filters (sentAt)
	if filters.DateRange != nil {
		dateFilter := bson.M{}
		if filters.DateRange.After != nil {
			dateFilter["$gte"] = *filters.DateRange.After
		}
		if filters.DateRange.Before != nil {
			dateFilter["$lt"] = *filters.DateRange.Before // Using $lt (exclusive) based on common date range semantics
		}
		if len(dateFilter) > 0 {
			andConditions = append(andConditions, bson.M{"sentAt": dateFilter})
		}
	}

	if len(andConditions) == 0 {
		return bson.M{}
	}
	if len(andConditions) == 1 {
		// If there's only one condition, return it directly instead of wrapping in $and
		return andConditions[0]
	}
	return bson.M{"$and": andConditions}
}

// buildThreadOverviewsFromAggregation processes raw BSON results from the
// fetchMailboxEntriesWithFilters aggregation pipeline into ThreadOverview structs.
func (m *MongoEmailService) buildThreadOverviewsFromAggregation(ctx context.Context, results []bson.M, userID, quillmail string) []ThreadOverview {
	if len(results) == 0 {
		return []ThreadOverview{}
	}

	// Extract thread IDs from the aggregated results for batch counting
	threadIDs := make([]string, 0, len(results))
	for _, result := range results {
		if threadID, ok := result["threadId"].(string); ok { // 'threadId' is directly available after replaceRoot
			threadIDs = append(threadIDs, threadID)
		}
	}

	// Batch count total and unread messages for these specific threads
	totalCounts, err := m.batchCountMessagesInThreads(ctx, threadIDs)
	if err != nil {
		log.Printf("Error batch counting total messages for overviews: %v", err)
		totalCounts = make(map[string]int64) // Initialize to avoid panic if error
	}

	unreadCounts, err := m.batchCountUnreadMessagesInThreads(ctx, quillmail, threadIDs)
	if err != nil {
		log.Printf("Error batch counting unread messages for overviews: %v", err)
		unreadCounts = make(map[string]int64) // Initialize to avoid panic if error
	}

	var threadOverviews []ThreadOverview
	for _, result := range results {
		threadID := getStringFromBson(result, "threadId")

		// 'messageDetails' is now a direct BSON map due to $unwind and $replaceRoot
		var messageDetails bson.M
		if details, ok := result["messageDetails"].(bson.M); ok {
			messageDetails = details
		} else {
			// This should ideally not happen if $match for non-empty messageDetails worked
			log.Printf("Warning: Aggregated result missing messageDetails for thread %s", threadID)
			continue
		}

		// 'options.read' (IsRead) comes from the original mailboxEntry
		isRead := false
		if options, ok := result["options"].(bson.M); ok {
			if read, ok := options["read"].(bool); ok {
				isRead = read
			}
		}
		isStared := false
		if result["options"].(bson.M)["starred"] != nil {
			isStared = result["options"].(bson.M)["starred"].(bool)
		} else {
			log.Printf("Warning: Aggregated result missing starred option for thread %s", threadID)

		}

		threadOverview := convertBsonToThreadOverview(messageDetails, isRead, isStared)

		// Populate counts
		threadOverview.Count = int(totalCounts[threadID])
		if threadOverview.Count == 0 { // Should at least be 1 if it passed filters
			threadOverview.Count = 1
		}
		threadOverview.UnreadCount = int(unreadCounts[threadID])

		threadOverviews = append(threadOverviews, threadOverview)
	}

	return threadOverviews
}

// Helper to count messages in a thread
func (m *MongoEmailService) countMessagesInThread(ctx context.Context, threadID string) (int64, error) {
	messageFilter := bson.M{
		"options.threadID": threadID,
	}
	return m.db.Collection("messages").CountDocuments(ctx, messageFilter)
}

// countUnreadMessagesInThread counts the number of unread messages for a user in a thread.
func (m *MongoEmailService) countUnreadMessagesInThread(ctx context.Context, userID string, threadID string) (int64, error) {
	filter := bson.M{
		"quillMail":    userID,
		"threadId":     threadID,
		"options.read": false,
	}
	count, err := m.db.Collection("mailboxes").CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// batchCountMessagesInThreads counts total messages for multiple threads in a single query
func (m *MongoEmailService) batchCountMessagesInThreads(ctx context.Context, threadIDs []string) (map[string]int64, error) {
	if len(threadIDs) == 0 {
		return make(map[string]int64), nil
	}

	pipeline := []bson.M{
		// Match messages for all thread IDs
		{"$match": bson.M{
			"options.threadID": bson.M{"$in": threadIDs},
		}},
		// Group by threadID and count messages
		{"$group": bson.M{
			"_id":   "$options.threadID",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := m.db.Collection("messages").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}()

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, result := range results {
		if threadID, ok := result["_id"].(string); ok {
			if count, ok := result["count"].(int32); ok {
				counts[threadID] = int64(count)
			}
		}
	}

	return counts, nil
}

// batchCountUnreadMessagesInThreads counts unread messages for multiple threads for a specific user in a single query
func (m *MongoEmailService) batchCountUnreadMessagesInThreads(ctx context.Context, userID string, threadIDs []string) (map[string]int64, error) {
	if len(threadIDs) == 0 {
		return make(map[string]int64), nil
	}

	pipeline := []bson.M{
		// Match unread mailbox entries for the user in the specified threads
		{"$match": bson.M{
			"quillMail":    userID,
			"threadId":     bson.M{"$in": threadIDs},
			"options.read": false,
		}},
		// Group by threadId and count unread messages
		{"$group": bson.M{
			"_id":   "$threadId",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := m.db.Collection("mailboxes").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}()

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for _, result := range results {
		if threadID, ok := result["_id"].(string); ok {
			if count, ok := result["count"].(int32); ok {
				counts[threadID] = int64(count)
			}
		}
	}

	return counts, nil
}
