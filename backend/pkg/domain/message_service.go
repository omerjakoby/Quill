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
}

// mailboxEntry represents a reference to a message in a user's mailbox
type mailboxEntry struct {
	UserID     string              `bson:"userId"`
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

	userID := req.From
	now := time.Now().UTC()

	messageDoc := bson.M{
		"messageId":   messageID,
		"fromID":      userID,
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
			UserID:     userID,
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
	if (req.Mode == FetchModeThread && req.ThreadID == nil) || (req.Mode == FetchModeOverview && req.Folder == nil) {
		return FetchEmailResult{}, errorString("missing required parameters for fetch mode")
	} else if req.Mode != FetchModeThread && req.Mode != FetchModeOverview {
		return FetchEmailResult{}, errorString("invalid fetch mode")
	} else if req.Mode == FetchModeOverview {
		return m.FetchFolder(ctx, req)
	} else if req.Mode == FetchModeThread {
		return m.FetchThread(ctx, req)
	}
	return FetchEmailResult{}, errorString("unsupported fetch mode")
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
	mailboxFilter := buildMailboxFilter(req, userID, quillmail)
	count, err := m.db.Collection("mailboxes").CountDocuments(ctx, mailboxFilter)
	if err != nil {
		return err
	}
	if count == 0 {
		// Also check with the quillmail address
		mailboxFilter = bson.M{
			"userId":   quillmail,
			"threadID": *req.ThreadID,
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
		"userId":    quillmail,
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

func (m *MongoEmailService) FetchFolder(ctx context.Context, req FetchEmailRequest) (FetchEmailResult, error) {
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

	filter := buildMailboxFilter(req, userID, quillmail)

	// Count unique threads instead of total messages
	total, err := m.countUniqueThreads(ctx, filter)
	if err != nil {
		return FetchEmailResult{}, err
	}

	entries, err := m.fetchMailboxEntries(ctx, filter, offset, limit)
	if err != nil {
		return FetchEmailResult{}, err
	}
	if len(entries) == 0 {
		return FetchEmailResult{
			TotalMessages: int(total),
			Limit:         limit,
			Offset:        offset,
			Messages:      []Message{},
		}, nil
	}

	messageIDs := extractMessageIDs(entries)
	messages, err := m.fetchMessagesByIDs(ctx, messageIDs, entries)
	if err != nil {
		return FetchEmailResult{}, err
	}

	return FetchEmailResult{
		TotalThreads:    int(total),
		Limit:           limit,
		Offset:          offset,
		ThreadOverviews: messages,
	}, nil
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

// buildMailboxFilter builds the filter for mailbox queries based on fetch mode
func buildMailboxFilter(req FetchEmailRequest, userID, quillmail string) bson.M {
	if req.Mode == FetchModeThread && req.ThreadID != nil {
		return bson.M{
			"userId":   quillmail,
			"threadId": *req.ThreadID,
		}
	} else if req.Mode == FetchModeOverview && req.Folder != nil {
		return bson.M{
			"userId": quillmail,
			"folder": *req.Folder,
		}
	}
	return bson.M{
		"userId": userID,
		"folder": "inbox",
	}
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
func (m *MongoEmailService) fetchMailboxEntries(ctx context.Context, filter bson.M, offset, limit int) ([]mailboxEntry, error) {
	// Use aggregation pipeline to get only the first message of each thread
	pipeline := []bson.M{
		// Match the filter criteria
		{"$match": filter},
		// Sort by receivedAt descending to get the latest message first within each thread
		{"$sort": bson.M{"receivedAt": -1}},
		// Group by threadId and take the first (latest) message of each thread
		{"$group": bson.M{
			"_id": "$threadId",
			"doc": bson.M{"$first": "$$ROOT"},
		}},
		// Replace the root document with the grouped document
		{"$replaceRoot": bson.M{"newRoot": "$doc"}},
		// Sort again by receivedAt to maintain chronological order across threads
		{"$sort": bson.M{"receivedAt": -1}},
		// Apply pagination
		{"$skip": offset},
		{"$limit": limit},
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

	var entries []mailboxEntry
	if err = cursor.All(ctx, &entries); err != nil {
		return nil, err
	}
	return entries, nil
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
func (m *MongoEmailService) fetchMessagesByIDs(ctx context.Context, messageIDs []string, entries []mailboxEntry) ([]ThreadOverview, error) {
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

	var messages []ThreadOverview
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, ErrUserNotAuthenticated
	}
	quillmail, err := m.getUserQuillMail(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Collect all unique thread IDs from entries
	threadIDSet := make(map[string]bool)
	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			threadID := getThreadIDFromBson(rawMsg)
			threadIDSet[threadID] = true
		}
	}

	// Convert to slice for batch queries
	var threadIDs []string
	for threadID := range threadIDSet {
		threadIDs = append(threadIDs, threadID)
	}

	// Batch query for total message counts per thread
	totalCounts, err := m.batchCountMessagesInThreads(ctx, threadIDs)
	if err != nil {
		log.Printf("Error batch counting total messages: %v", err)
		// Fallback to empty map - individual counts will default to 1
		totalCounts = make(map[string]int64)
	}

	// Batch query for unread message counts per thread for this user
	unreadCounts, err := m.batchCountUnreadMessagesInThreads(ctx, quillmail, threadIDs)
	if err != nil {
		log.Printf("Error batch counting unread messages: %v", err)
		// Fallback to empty map - individual counts will default to 0
		unreadCounts = make(map[string]int64)
	}

	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			threadID := getThreadIDFromBson(rawMsg)

			// Use pre-fetched counts, with fallback defaults
			totalCount, exists := totalCounts[threadID]
			if !exists {
				totalCount = 1
			}

			unreadCount, exists := unreadCounts[threadID]
			if !exists {
				unreadCount = 0
			}

			message := convertBsonToThreadOverview(rawMsg, entry.Options.Read)
			message.Count = int(totalCount)
			message.UnreadCount = int(unreadCount)

			messages = append(messages, message)
		}
	}
	return messages, nil
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
func convertBsonToThreadOverview(bsonMsg bson.M, read bool) ThreadOverview {
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
				IsRead: read,
			},
		},
	}
	return msg
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

	return UpdateEmailResult{}, errorString("UpdateEmail not implemented")

}

// countUniqueThreads counts the number of unique threads matching the filter
func (m *MongoEmailService) countUniqueThreads(ctx context.Context, filter bson.M) (int64, error) {
	pipeline := []bson.M{
		// Match the filter criteria
		{"$match": filter},
		// Group by threadId to get unique threads
		{"$group": bson.M{
			"_id": "$threadId",
		}},
		// Count the number of unique threads
		{"$count": "totalThreads"},
	}

	cursor, err := m.db.Collection("mailboxes").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}()

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	if count, ok := result[0]["totalThreads"].(int32); ok {
		return int64(count), nil
	}

	return 0, nil
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
		"userId":       userID,
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
			"userId":       userID,
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
