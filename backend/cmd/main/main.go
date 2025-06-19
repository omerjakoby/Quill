package main

import (
	"context"
	"github.com/joho/godotenv"
	"log"
	"os"
	"quill/pkg/db"
	"quill/pkg/domain"
	"quill/pkg/transport/quill"
	"strings"
	"time"
)

func main() {
	log.Println("Starting Quill server...")
	authSvc, err := quill.InitAuthServiceFromEnv(context.Background(), "../.env")
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}

	// Get MongoDB connection details from environment
	mongoURI := getEnvWithDefault("MONGODB_URI", "mongodb://localhost:27017")
	mongoPassword := getEnvWithDefault("mongodb_password", "")

	// Use a specific database name, not the full connection string
	mongoDatabase := getEnvWithDefault("MONGODB_DATABASE", "quill")

	// Replace password placeholder in connection string if needed
	if mongoPassword != "" && strings.Contains(mongoURI, "<db_password>") {
		mongoURI = strings.Replace(mongoURI, "<db_password>", mongoPassword, 1)
	}

	// Initialize MongoDB connection
	mongoConfig := db.MongoConfig{
		URI:      mongoURI,
		Database: mongoDatabase, // Use the database name only, not the full URI
		Timeout:  10 * time.Second,
	}

	log.Printf("Connecting to MongoDB: %s (database: %s)", maskConnectionString(mongoURI), mongoDatabase)

	mongoDB, err := db.NewMongoDB(mongoConfig)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB successfully")

	// Create message service using MongoDB
	msgSvc := domain.NewMongoMessageService(
		mongoDB.GetMessagesCollection(),
		mongoDB.GetMailboxesCollection(),
	)
	log.Println("Created MongoDB-backed message service")

	messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

	serverAddr := "localhost:9876"
	server := quill.NewServer(serverAddr, messageHandler)

	log.Printf("INFO: starting Quill protocol server on %s", serverAddr)
	/*if err := server.Start(); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
	*/

	if err := server.StartTLS("../certificate/quill.crt", "../certificate/quill.key"); err != nil {
		log.Fatalf("FATAL: could not start TLS server: %v", err)
	}
}

// Helper function to get environment variable with fallback default
func getEnvWithDefault(key, defaultValue string) string {
	_ = godotenv.Load("../.env") // Loads .env file if present, ignores error if not
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to mask sensitive parts of the connection string for logging
func maskConnectionString(uri string) string {
	if strings.Contains(uri, "@") {
		parts := strings.Split(uri, "@")
		if len(parts) > 1 {
			authPart := parts[0]
			hostPart := parts[1]

			// Mask the password in the auth part
			if strings.Contains(authPart, ":") {
				authParts := strings.Split(authPart, ":")
				if len(authParts) > 1 {
					// Replace with protocol://username:****
					return authParts[0] + ":****@" + hostPart
				}
			}
		}
	}
	return strings.Replace(uri, "mongodb+srv://", "mongodb+srv://*****", 1)
}
