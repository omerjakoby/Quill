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

// MessageService defines the interface for message-related operations
type MessageService interface {
	Send(ctx context.Context, req DomainSendRequest) (DomainSendResult, error)
	Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error)
}

// MockMessageService implements the MessageService interface with mock data

type MockMessageService struct{}

/**
// Send implements a mock message sending operation
func (m *MockMessageService) Send(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	log.Println("Mock: Sending message to", req.To)
	return DomainSendResult{
		MessageID:   "mock-msg-123",
		ThreadID:    "mock-thread-456",
		DeliveredTo: req.To,
		QueuedFor:   []string{},
	}, nil
}

 Fetch implements a mock message fetching operation
func (m *MockMessageService) Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	log.Println("Mock: Fetching messages with mode", req.Mode)
	return DomainFetchResult{
		Total:    0,
		Limit:    10,
		Offset:   0,
		Messages: []Message{},
	}, nil
}
**/

// MongoMessageService implements the MessageService interface with MongoDB storage
type MongoMessageService struct {
	db *mongo.Database
}

// NewMongoMessageService creates a new MongoDB-backed MessageService
func NewMongoMessageService(db *mongo.Database) *MongoMessageService {
	return &MongoMessageService{
		db: db,
	}
}

// mailboxEntry represents a reference to a message in a user's mailbox
type mailboxEntry struct {
	UserID     string    `bson:"userId"`
	MessageID  string    `bson:"messageId"`
	ThreadID   string    `bson:"threadId"`
	Folder     string    `bson:"folder"`
	Read       bool      `bson:"read"`
	ReceivedAt time.Time `bson:"receivedAt"`
	Category   string    `bson:"category,omitempty"` // Optional category field
}

// Send stores a message in MongoDB and adds entries to each recipient's mailbox
func (m *MongoMessageService) Send(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	// Validate the request
	if validateQuillMailFormat(req.From) {
		if extractDomain(req.From) == constants.DOMAIN_NAME {
			return m.SendInternal(ctx, req)
		}
		return m.SendExternal(ctx, req)
	}
	return DomainSendResult{}, errorString("invalid sender address format")

}

func (m *MongoMessageService) SendInternal(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {

	// Validate recipient address format
	for _, addr := range append(append(req.To, req.CC...), req.BCC...) {
		if !validateQuillMailFormat(addr) {
			return DomainSendResult{}, errorString(fmt.Sprintf("invalid recipient address format: %s", addr))
		}
	}

	messageID, err := getOrValidateMessageID(req.MessageID)
	if err != nil {
		return DomainSendResult{}, err
	}

	threadID, err := getOrValidateThreadID(req.Options.ThreadID)
	if err != nil {
		return DomainSendResult{}, err
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
		return DomainSendResult{}, err
	}

	entries := []interface{}{
		mailboxEntry{
			UserID:     userID,
			MessageID:  messageID,
			ThreadID:   threadID,
			Folder:     "sent",
			Read:       true,
			ReceivedAt: now,
		},
	}

	category, err := m.GetCategory(ctx, req)
	if err != nil {
		log.Printf("Failed to get category for message: %v", err)
		return DomainSendResult{}, err
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

	return DomainSendResult{
		MessageID:   messageID,
		ThreadID:    threadID,
		DeliveredTo: internal,
		QueuedFor:   external,
	}, nil
}

func (m *MongoMessageService) SendExternal(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	// Validate and extract threadID and messageID
	threadID, err := validateThreadID(req.Options.ThreadID)
	if err != nil {
		return DomainSendResult{}, err
	}
	messageID, err := validateMessageID(req.MessageID)
	if err != nil {
		return DomainSendResult{}, err
	}

	for _, addr := range append(append(req.To, req.CC...), req.BCC...) {
		if !validateQuillMailFormat(addr) {
			return DomainSendResult{}, errorString(fmt.Sprintf("invalid recipient address format: %s", addr))
		}
	}

	// Check for existing message
	exists, err := m.messageExists(ctx, messageID)
	if err != nil {
		return DomainSendResult{}, err
	}
	if exists {
		return DomainSendResult{}, errorString("message with this ID already exists")
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
		return DomainSendResult{}, err
	}

	category, err := m.GetCategory(ctx, req)
	if err != nil {
		log.Printf("Failed to get category for message: %v", err)
		return DomainSendResult{}, err
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

	return DomainSendResult{
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
func (m *MongoMessageService) messageExists(ctx context.Context, messageID string) (bool, error) {
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
func getInternalRecipients(req DomainSendRequest, domain string) []string {
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
			Read:       false,
			ReceivedAt: now,
			Category:   category,
		})
	}
	return entries
}

// Fetch retrieves messages based on the provided request
func (m *MongoMessageService) Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	if (req.Mode == FetchModeThread && req.ThreadID == nil) || (req.Mode == FetchModeFolder && req.Folder == nil) {
		return DomainFetchResult{}, errorString("missing required parameters for fetch mode")
	} else if req.Mode != FetchModeThread && req.Mode != FetchModeFolder {
		return DomainFetchResult{}, errorString("invalid fetch mode")
	} else if req.Mode == FetchModeFolder {
		return m.FetchFolder(ctx, req)
	} else if req.Mode == FetchModeThread {
		return m.FetchThread(ctx, req)
	}
	return DomainFetchResult{}, errorString("unsupported fetch mode")
}

func (m *MongoMessageService) FetchThread(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return DomainFetchResult{}, ErrUserNotAuthenticated
	}

	if req.ThreadID == nil {
		return DomainFetchResult{}, errorString("thread ID is required for thread mode")
	}

	quillmail, err := m.getUserQuillMail(ctx, userID)
	if err != nil {
		return DomainFetchResult{}, err
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
		return DomainFetchResult{}, err
	}

	// Get all messages in the thread
	rawMessages, total, err := m.fetchThreadMessages(ctx, *req.ThreadID, offset, limit)
	if err != nil {
		return DomainFetchResult{}, err
	}

	if len(rawMessages) == 0 {
		return DomainFetchResult{
			Total:    int(total),
			Limit:    limit,
			Offset:   offset,
			Messages: []Message{},
		}, nil
	}

	messageIDs := extractMessageIDsFromRaw(rawMessages)
	readStatusMap, err := m.fetchReadStatusMap(ctx, quillmail, messageIDs)
	if err != nil {
		return DomainFetchResult{}, err
	}

	messages := mapRawMessagesToDomain(rawMessages, readStatusMap)

	return DomainFetchResult{
		Total:    int(total),
		Limit:    limit,
		Offset:   offset,
		Messages: messages,
	}, nil
}

// Helper: Check if user has access to the thread
func (m *MongoMessageService) checkThreadAccess(ctx context.Context, req DomainFetchRequest, userID, quillmail string) error {
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
func (m *MongoMessageService) fetchThreadMessages(ctx context.Context, threadID string, offset, limit int) ([]bson.M, int64, error) {
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
func (m *MongoMessageService) fetchReadStatusMap(ctx context.Context, quillmail string, messageIDs []string) (map[string]bool, error) {
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
		readStatusMap[entry.MessageID] = entry.Read
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

func (m *MongoMessageService) FetchFolder(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return DomainFetchResult{}, ErrUserNotAuthenticated
	}

	quillmail, err := m.getUserQuillMail(ctx, userID)
	if err != nil {
		return DomainFetchResult{}, err
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
	total, err := m.db.Collection("mailboxes").CountDocuments(ctx, filter)
	if err != nil {
		return DomainFetchResult{}, err
	}

	entries, err := m.fetchMailboxEntries(ctx, filter, offset, limit)
	if err != nil {
		return DomainFetchResult{}, err
	}
	if len(entries) == 0 {
		return DomainFetchResult{
			Total:    int(total),
			Limit:    limit,
			Offset:   offset,
			Messages: []Message{},
		}, nil
	}

	messageIDs := extractMessageIDs(entries)
	messages, err := m.fetchMessagesByIDs(ctx, messageIDs, entries)
	if err != nil {
		return DomainFetchResult{}, err
	}

	return DomainFetchResult{
		Total:    int(total),
		Limit:    limit,
		Offset:   offset,
		Messages: messages,
	}, nil
}

// getUserQuillMail retrieves the user's quillmail address from the users collection
func (m *MongoMessageService) getUserQuillMail(ctx context.Context, userID string) (string, error) {
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
func buildMailboxFilter(req DomainFetchRequest, userID, quillmail string) bson.M {
	if req.Mode == FetchModeThread && req.ThreadID != nil {
		return bson.M{
			"userId":   quillmail,
			"threadId": *req.ThreadID,
		}
	} else if req.Mode == FetchModeFolder && req.Folder != nil {
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

func (m *MongoMessageService) GetCategory(ctx context.Context, req DomainSendRequest) (string, error) {
	var htmlContent string
	for _, content := range req.Body.Content {
		if content.Type == ContentTypeHTML {
			htmlContent = content.Value
			break
		} else {
			htmlContent = content.Value
		}
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
			panic("unhandled default case")
		}
	}

CLEANUP:
	// Normalize whitespace: collapse runs of space to one, trim ends.
	fields := strings.Fields(buf.String())
	return strings.Join(fields, " "), nil
}

// fetchMailboxEntries retrieves mailbox entries with sorting, skip, and limit
func (m *MongoMessageService) fetchMailboxEntries(ctx context.Context, filter bson.M, offset, limit int) ([]mailboxEntry, error) {
	findOptions := options.Find().
		SetSort(bson.D{{Key: "receivedAt", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := m.db.Collection("mailboxes").Find(ctx, filter, findOptions)
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
func (m *MongoMessageService) fetchMessagesByIDs(ctx context.Context, messageIDs []string, entries []mailboxEntry) ([]Message, error) {
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

	var messages []Message
	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			message := convertBsonToMessage(rawMsg, entry.Read)
			messages = append(messages, message)
		}
	}
	return messages, nil
}

// Helper function to convert BSON to Message domain object
func convertBsonToMessage(bsonMsg bson.M, read bool) Message {
	msg := Message{
		MessageID: getStringFromBson(bsonMsg, "messageId"),
		ThreadID:  getThreadIDFromBson(bsonMsg),
		From:      getStringFromBson(bsonMsg, "fromID"),
		Read:      read,
	}

	msg.To = getStringArrayFromBson(bsonMsg, "to")
	msg.CC = getStringArrayFromBson(bsonMsg, "cc")
	msg.Subject = getStringFromBson(bsonMsg, "subject")
	msg.SentAt = getTimeFromBson(bsonMsg, "sentAt")
	msg.Body = getBodyFromBson(bsonMsg)

	return msg
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

func getBodyFromBson(bsonMsg bson.M) Body {
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

// Helper to parse a content item from BSON
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
				Read:       false,
				ReceivedAt: now,
				Category:   category,
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
