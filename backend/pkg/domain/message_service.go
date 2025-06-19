package domain

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
	messagesCollection *mongo.Collection
	mailboxCollection  *mongo.Collection
}

// NewMongoMessageService creates a new MongoDB-backed MessageService
func NewMongoMessageService(messagesCollection, mailboxCollection *mongo.Collection) *MongoMessageService {
	return &MongoMessageService{
		messagesCollection: messagesCollection,
		mailboxCollection:  mailboxCollection,
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
	// Extract sender ID from context
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return DomainSendResult{}, ErrUserNotAuthenticated
	}

	// Generate new message ID and thread ID if not provided
	messageID := uuid.New().String()
	threadID := messageID
	if req.Options.ThreadID != nil && *req.Options.ThreadID != "" {
		threadID = *req.Options.ThreadID
	}

	// Prepare message document
	now := time.Now().UTC()
	messageDoc := bson.M{
		"messageId":   messageID,
		"threadId":    threadID,
		"from":        userID,
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
		},
	}

	// Insert message into messages collection
	_, err := m.messagesCollection.InsertOne(ctx, messageDoc)
	if err != nil {
		log.Printf("Failed to insert message: %v", err)
		return DomainSendResult{}, err
	}

	// Create mailbox entries for all recipients (including sender's sent folder)
	var mailboxEntries []interface{}

	// Add entry for sender (in "sent" folder)
	mailboxEntries = append(mailboxEntries, mailboxEntry{
		UserID:     userID,
		MessageID:  messageID,
		ThreadID:   threadID,
		Folder:     "sent",
		Read:       true,
		ReceivedAt: now,
	})
	// TODO send messages to the recipients
	/*
		// Add entries for recipients (in "inbox" folder) (optional)
		// maybe we should do this
		allRecipients := append([]string{}, req.To...)
		allRecipients = append(allRecipients, req.CC...)
		allRecipients = append(allRecipients, req.BCC...)

		for _, recipient := range allRecipients {
			mailboxEntries = append(mailboxEntries, mailboxEntry{
				UserID:     recipient,
				MessageID:  messageID,
				ThreadID:   threadID,
				Folder:     "inbox",
				Read:       false,
				ReceivedAt: now,
			})
		}
	*/
	// Insert all mailbox entries
	if len(mailboxEntries) > 0 {
		_, err = m.mailboxCollection.InsertMany(ctx, mailboxEntries)
		if err != nil {
			log.Printf("Failed to insert mailbox entries: %v", err)
			// Consider handling this error (perhaps delete the message?)
		}
	}

	return DomainSendResult{
		MessageID:   messageID,
		ThreadID:    threadID,
		DeliveredTo: req.To,
		QueuedFor:   []string{}, // In a real implementation, this might contain addresses that couldn't be delivered immediately
	}, nil
}

// Fetch retrieves messages based on the provided request
func (m *MongoMessageService) Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	// Extract user ID from context
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return DomainFetchResult{}, ErrUserNotAuthenticated
	}

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
	var filter bson.M
	if req.Mode == FetchModeThread && req.ThreadID != nil {
		filter = bson.M{
			"userId":   userID,
			"threadId": *req.ThreadID,
		}
	} else if req.Mode == FetchModeFolder && req.Folder != nil {
		filter = bson.M{
			"userId": userID,
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
	total, err := m.mailboxCollection.CountDocuments(ctx, filter)
	if err != nil {
		return DomainFetchResult{}, err
	}

	// Find mailbox entries
	findOptions := options.Find().
		SetSort(bson.D{{Key: "receivedAt", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := m.mailboxCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return DomainFetchResult{}, err
	}
	defer cursor.Close(ctx)

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
	messageCursor, err := m.messagesCollection.Find(ctx, messageFilter)
	if err != nil {
		return DomainFetchResult{}, err
	}
	defer messageCursor.Close(ctx)

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
