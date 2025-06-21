package domain

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
}

// Send stores a message in MongoDB and adds entries to each recipient's mailbox
func (m *MongoMessageService) Send(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	if extractDomain(req.From) == constants.DOMAIN_NAME {
		return m.SendInternal(ctx, req)
	}
	return m.SendExternal(ctx, req)
}

func (m *MongoMessageService) SendInternal(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	// 1. Validate or generate messageID
	var messageID string
	if req.MessageID != "" {
		if !isUUID(req.MessageID) {
			return DomainSendResult{},
				errorString("invalid message ID: must be a UUID")
		}
		messageID = req.MessageID
	} else {
		messageID = uuid.New().String()
	}

	// 2. Validate or generate threadID
	var threadID string
	if req.Options.ThreadID != nil && *req.Options.ThreadID != "" {
		if !isUUID(*req.Options.ThreadID) {
			return DomainSendResult{},
				errorString("invalid thread ID: must be a UUID")
		}
		threadID = *req.Options.ThreadID
	} else {
		threadID = uuid.New().String()
	}

	// From is always within our domain here
	userID := req.From
	now := time.Now().UTC()

	// Prepare the message document
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

	// Insert into messages collection
	if _, err := m.db.Collection("messages").
		InsertOne(ctx, messageDoc); err != nil {
		log.Printf("Failed to insert message: %v", err)
		return DomainSendResult{}, err
	}

	// Build mailbox entries
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

	allRecipients := append(append(req.To, req.CC...), req.BCC...)
	var internal, external []string
	for _, addr := range allRecipients {
		if strings.HasSuffix(addr, constants.DOMAIN_NAME) {
			internal = append(internal, addr)
			entries = append(entries, mailboxEntry{
				UserID:     addr,
				MessageID:  messageID,
				ThreadID:   threadID,
				Folder:     "inbox",
				Read:       false,
				ReceivedAt: now,
			})
		} else {
			external = append(external, addr)
		}
	}

	if len(entries) > 0 {
		if _, err := m.db.Collection("mailboxes").
			InsertMany(ctx, entries); err != nil {
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

	// Generate new message ID and thread ID if not provided
	myRecipients := []string{}
	for _, addr := range req.To {
		if extractDomain(addr) == constants.DOMAIN_NAME {
			myRecipients = append(myRecipients, addr)
		}
	}
	for _, addr := range req.CC {
		if extractDomain(addr) == constants.DOMAIN_NAME {
			myRecipients = append(myRecipients, addr)
		}
	}
	for _, addr := range req.BCC {
		if extractDomain(addr) == constants.DOMAIN_NAME {
			myRecipients = append(myRecipients, addr)
		}
	}

	threadID := *req.Options.ThreadID
	if !(req.Options.ThreadID != nil && *req.Options.ThreadID != "") || isUUID(*req.Options.ThreadID) == false {
		return DomainSendResult{}, errorString("did not provide thread ID")
	}
	messageID := req.MessageID
	if !(req.MessageID != "") {
		return DomainSendResult{}, errorString("did not provide message ID")
	}
	// needs to be indexed for better performance TODO: add index on messageId
	singleResult := m.db.Collection("messages").FindOne(ctx, bson.M{"messageId": messageID})
	if err := singleResult.Err(); err != nil {
		if err != mongo.ErrNoDocuments {
			log.Printf("failed to check if message exists: %v", err)
			return DomainSendResult{}, err
		}
	} else {
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
	_, err := m.db.Collection("messages").InsertOne(ctx, messageDoc)
	if err != nil {
		log.Printf("Failed to insert message: %v", err)
		return DomainSendResult{}, err
	}

	// Create mailbox entries for all recipients (including sender's sent folder)
	var mailboxEntries []interface{}

	for _, recipient := range myRecipients {
		mailboxEntries = append(mailboxEntries, mailboxEntry{
			UserID:     recipient,
			MessageID:  messageID,
			ThreadID:   threadID,
			Folder:     "inbox",
			Read:       false,
			ReceivedAt: now,
		})
	}
	// Insert all mailbox entries
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

// Fetch retrieves messages based on the provided request
func (m *MongoMessageService) Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	// Extract user ID from context
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return DomainFetchResult{}, ErrUserNotAuthenticated
	}
	// get users quillmail domain

	collection := m.db.Collection("users")
	filter := bson.M{"_id": userID}
	var result struct {
		UserQuillMail string `bson:"userQuillMail"`
	}
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return DomainFetchResult{}, err
		}
		return DomainFetchResult{}, fmt.Errorf("error retrieving userQuillMail: %w", err)
	}
	quillmail := result.UserQuillMail

	// Set default limit and offset if not provided
	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
	}

	offset := 0
	if req.Offset != nil {
		offset = *req.Offset
	}

	// Build query based on fetch mode
	if req.Mode == FetchModeThread && req.ThreadID != nil {
		filter = bson.M{
			"userId":           userID,
			"options.threadID": *req.ThreadID,
		}
	} else if req.Mode == FetchModeFolder && req.Folder != nil {
		filter = bson.M{
			"userId": quillmail,
			"folder": *req.Folder,
		}
	} else {
		// Default to inbox if no valid mode/parameters provided
		filter = bson.M{
			"userId": userID,
			"folder": "inbox",
		}
	}

	// Get total count of matching messages
	total, err := m.db.Collection("mailboxes").CountDocuments(ctx, filter)
	if err != nil {
		return DomainFetchResult{}, err
	}

	// Find mailbox entries
	findOptions := options.Find().
		SetSort(bson.D{{Key: "receivedAt", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := m.db.Collection("mailboxes").Find(ctx, filter, findOptions)
	if err != nil {
		return DomainFetchResult{}, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error closing cursor: %v", err)
		}
	}()

	// Collect message IDs from mailbox entries
	var entries []mailboxEntry
	if err = cursor.All(ctx, &entries); err != nil {
		return DomainFetchResult{}, err
	}

	if len(entries) == 0 {
		// No messages found
		return DomainFetchResult{
			Total:    int(total),
			Limit:    limit,
			Offset:   offset,
			Messages: []Message{},
		}, nil
	}

	// Extract message IDs to fetch actual messages
	var messageIDs []string
	for _, entry := range entries {
		messageIDs = append(messageIDs, entry.MessageID)
	}

	// Fetch the actual messages
	messageFilter := bson.M{"messageId": bson.M{"$in": messageIDs}}
	messageCursor, err := m.db.Collection("messages").Find(ctx, messageFilter)
	if err != nil {
		return DomainFetchResult{}, err
	}
	defer func() {
		if err := messageCursor.Close(ctx); err != nil {
			log.Printf("Error closing message cursor: %v", err)
		}
	}()

	// Map to store messages by ID for quick lookup
	messageMap := make(map[string]bson.M)
	var rawMessages []bson.M
	if err = messageCursor.All(ctx, &rawMessages); err != nil {
		return DomainFetchResult{}, err
	}

	for _, msg := range rawMessages {
		if msgID, ok := msg["messageId"].(string); ok {
			messageMap[msgID] = msg
		}
	}

	// Convert to domain Message objects in the correct order
	var messages []Message
	for _, entry := range entries {
		if rawMsg, found := messageMap[entry.MessageID]; found {
			message := convertBsonToMessage(rawMsg, entry.Read)
			messages = append(messages, message)
		}
	}

	return DomainFetchResult{
		Total:    int(total),
		Limit:    limit,
		Offset:   offset,
		Messages: messages,
	}, nil
}

// Helper function to convert BSON to Message domain object
func convertBsonToMessage(bsonMsg bson.M, read bool) Message {
	// This is a simplified conversion - in a real implementation you'd need to handle all fields properly
	msg := Message{
		MessageID: bsonMsg["messageId"].(string),
		ThreadID:  bsonMsg["threadId"].(string),
		From:      bsonMsg["from"].(string),
		Read:      read,
	}

	// Handle array fields
	if to, ok := bsonMsg["to"].([]interface{}); ok {
		for _, t := range to {
			if str, ok := t.(string); ok {
				msg.To = append(msg.To, str)
			}
		}
	}

	if cc, ok := bsonMsg["cc"].([]interface{}); ok {
		for _, c := range cc {
			if str, ok := c.(string); ok {
				msg.CC = append(msg.CC, str)
			}
		}
	}

	if bcc, ok := bsonMsg["bcc"].([]interface{}); ok {
		for _, b := range bcc {
			if str, ok := b.(string); ok {
				msg.BCC = append(msg.BCC, str)
			}
		}
	}

	// Handle subject
	if subj, ok := bsonMsg["subject"].(string); ok {
		msg.Subject = subj
	}

	// Handle sentAt
	if sentAt, ok := bsonMsg["sentAt"].(time.Time); ok {
		msg.SentAt = sentAt
	}

	// Other fields would be converted similarly
	// This is simplified for brevity

	return msg
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
