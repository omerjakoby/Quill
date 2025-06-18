package main

import (
	"context"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"log"
	"os"
	"quill/pkg/domain"
	"quill/pkg/transport/quill"
)

// MockMessageService implements the messageService interface with dummy functionality
type MockMessageService struct{}

func (m *MockMessageService) Send(ctx context.Context, req domain.DomainSendRequest) (domain.DomainSendResult, error) {
	log.Println("Mock: Sending message to", req.To)
	return domain.DomainSendResult{
		MessageID:   "mock-msg-123",
		ThreadID:    "mock-thread-456",
		DeliveredTo: req.To,
		QueuedFor:   []string{},
	}, nil
}

func (m *MockMessageService) Fetch(ctx context.Context, req domain.DomainFetchRequest) (domain.DomainFetchResult, error) {
	log.Println("Mock: Fetching messages with mode", req.Mode)
	return domain.DomainFetchResult{
		Total:    0,
		Limit:    10,
		Offset:   0,
		Messages: []domain.Message{},
	}, nil
}

func main() {
	log.Println("Starting Quill server...")

	// --- Step 1: Initialize Firebase Admin SDK ---
	// This is the first crucial step. You need to provide the path to your
	// Firebase service account key.
	//
	// Security Note: NEVER hardcode this path or commit the file directly to Git.
	// For production, always load it from an environment variable.
	//
	// Example using environment variable:
	err := godotenv.Load("C:\\Users\\assij\\GolandProjects\\Quill\\.env") // Assuming main.go is in backend/cmd/main/
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	serviceAccountKeyPath := os.Getenv("firebase_service_account_path")
	if serviceAccountKeyPath == "" {
		log.Fatal("firebase_service_account_path environment variable not set.")
		log.Fatalf("Fatal error: FIREBASE_SERVICE_ACCOUNT_KEY_PATH environment variable not set.")
	}

	fmt.Println("Firebase Service Account Key Path:", serviceAccountKeyPath)

	sa := option.WithCredentialsFile(serviceAccountKeyPath)

	// Create a background context for Firebase initialization
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		log.Fatalf("Fatal error: cannot initialize Firebase app: %v", err)
	}
	firebaseAuthClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("Fatal error: cannot get Firebase Auth client: %v", err)
	}

	log.Println("Firebase Admin SDK initialized successfully.")

	// --- Instantiate your FirebaseAuthService ---
	// This service now wraps the Firebase Auth client.
	authSvc := quill.NewFirebaseAuthService(firebaseAuthClient)

	// Create the mock message service
	msgSvc := &MockMessageService{}
	log.Println("Created mock message service for testing")

	messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

	serverAddr := "localhost:9876"
	server := quill.NewServer(serverAddr, messageHandler)

	log.Printf("INFO: starting Quill protocol server on %s", serverAddr)
	if err := server.Start(); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
}
