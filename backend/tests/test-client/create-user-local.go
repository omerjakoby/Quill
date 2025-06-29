package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- models/user.go content ---

// User struct defines the structure of a user document in MongoDB.
// UsersUID is mapped to "_id" making it the primary key.
type User struct {
	UsersUID      string `bson:"_id"`           // Maps UsersUID directly to MongoDB's _id
	UserQuillMail string `bson:"userQuillMail"` // Unique Quill Mail
	UserEmail     string `bson:"userEmail"`     // User's regular email
	// Add any other user fields here as needed
}

// --- db/mongodb.go content (modified slightly for single file context) ---

// MongoConfig holds the configuration for the MongoDB connection.
type MongoConfig struct {
	URI        string
	Database   string
	Collection string // Not strictly used by NewMongoDB, but kept for consistency
	Timeout    time.Duration
}

// MongoDB is a client wrapper for MongoDB operations.
type MongoDB struct {
	client    *mongo.Client
	database  *mongo.Database
	messages  *mongo.Collection
	mailboxes *mongo.Collection
	users     *mongo.Collection // This is the collection we'll use for users
	config    MongoConfig
}

// NewMongoDB creates a new MongoDB connection.
func NewMongoDB(config MongoConfig) (*MongoDB, error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the MongoDB server to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		// Disconnect if ping fails to clean up connection resources
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx) // Ignore error on disconnect during failed connect
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(config.Database)

	return &MongoDB{
		client:    client,
		database:  db,
		messages:  db.Collection("messages"),
		mailboxes: db.Collection("mailboxes"),
		users:     db.Collection("users"), // Initialize users collection
		config:    config,
	}, nil
}

// Close closes the MongoDB connection.
func (m *MongoDB) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// GetMessagesCollection returns the messages collection.
func (m *MongoDB) GetMessagesCollection() *mongo.Collection {
	return m.messages
}

// GetMailboxesCollection returns the user mailboxes collection.
func (m *MongoDB) GetMailboxesCollection() *mongo.Collection {
	return m.mailboxes
}

// GetUsersCollection returns the users collection.
func (m *MongoDB) GetUsersCollection() *mongo.Collection {
	return m.users
}

// GetDatabase returns the underlying MongoDB database.
func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.database
}

// --- Main Program Logic ---

func main() {
	// Ensure your local MongoDB Docker container is running:
	// docker run -d --name my-mongo-db -p 27017:27017 mongo:latest

	// 1. Configure MongoDB connection for local Docker
	mongoConfig := MongoConfig{
		URI:      "mongodb://localhost:27017", // Connect to local Docker MongoDB
		Database: "quill_mail",                // You can change this database name
		Timeout:  5 * time.Second,             // Quick timeout for local connection
	}

	mongoDB, err := NewMongoDB(mongoConfig)
	if err != nil {
		log.Fatalf("Failed to connect to local MongoDB: %v", err)
	}
	defer func() {
		// It's good practice to ensure the context for Close is distinct and has its own timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := mongoDB.Close(ctx); err != nil {
			log.Printf("Error closing MongoDB connection: %v", err)
		}
	}()

	fmt.Println("Successfully connected to local MongoDB!")

	// 2. Get the users collection
	usersCollection := mongoDB.GetUsersCollection()

	// Optional: Create a unique index on 'userQuillMail' if it doesn't exist.
	// This ensures that userQuillMail values are unique across documents.
	// You typically do this once in your application setup, not on every run.
	indexModel := mongo.IndexModel{
		Keys:    primitive.D{{Key: "userQuillMail", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	ctxIndex, cancelIndex := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelIndex()
	_, err = usersCollection.Indexes().CreateOne(ctxIndex, indexModel)
	if err != nil {
		// Log the error but don't fatally exit, as the index might already exist
		fmt.Printf("Warning: Could not create unique index on 'userQuillMail': %v (might already exist)\n", err)
	} else {
		fmt.Println("Ensured unique index on 'userQuillMail'.")
	}

	// 3. Define the user data using the specific values provided
	testUID := "ftjphktn8mfiG4TWD1voZi4GCsU2"
	testQuillMail := "omer~quillmail.xyz"
	testEmail := "omer.jakoby@gmail.com"

	newUser := &User{
		UsersUID:      testUID,
		UserQuillMail: testQuillMail,
		UserEmail:     testEmail,
	}

	fmt.Printf("\nAttempting to insert user:\n  UID: %s\n  Quill Mail: %s\n  Email: %s\n",
		newUser.UsersUID, newUser.UserQuillMail, newUser.UserEmail)

	// 4. Insert the user document directly
	// Use a context for the insert operation
	insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer insertCancel()

	_, err = usersCollection.InsertOne(insertCtx, newUser)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			fmt.Printf("User insertion failed: Duplicate key error!\nUser with UID '%s' or Quill Mail '%s' likely already exists.\n",
				newUser.UsersUID, newUser.UserQuillMail)
		} else {
			log.Fatalf("Failed to insert user document: %v", err)
		}
	} else {
		fmt.Println("User document inserted successfully!")
	}

	fmt.Println("\nTest finished. You can verify manually:")
	fmt.Printf("  1. Connect to your MongoDB shell: `docker exec -it my-mongo-db mongosh %s`\n", mongoConfig.Database)
	fmt.Println("  2. Then run: `db.users.find().pretty()`")
}
