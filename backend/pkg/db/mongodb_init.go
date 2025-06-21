package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB configuration
type MongoConfig struct {
	URI        string
	Database   string
	Collection string
	Timeout    time.Duration
}

// MongoDB client wrapper
type MongoDB struct {
	client    *mongo.Client
	database  *mongo.Database
	messages  *mongo.Collection
	mailboxes *mongo.Collection
	users     *mongo.Collection
	config    MongoConfig
}

// NewMongoDB creates a new MongoDB connection
func NewMongoDB(config MongoConfig) (*MongoDB, error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	db := client.Database(config.Database)

	return &MongoDB{
		client:    client,
		database:  db,
		messages:  db.Collection("messages"),
		mailboxes: db.Collection("mailboxes"),
		users:     db.Collection("users"),
		config:    config,
	}, nil
}

// Close closes the MongoDB connection
func (m *MongoDB) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// GetMessagesCollection returns the messages collection
func (m *MongoDB) GetMessagesCollection() *mongo.Collection {
	return m.messages
}

// GetMailboxesCollection returns the user mailboxes collection
func (m *MongoDB) GetMailboxesCollection() *mongo.Collection {
	return m.mailboxes
}

// GetUsersCollection returns the users collection
func (m *MongoDB) GetUsersCollection() *mongo.Collection {
	return m.users
}

// GetDatabase returns the underlying MongoDB database
func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.database
}
