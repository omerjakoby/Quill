package main

import (
	"context"
	"log"
	"os"
	"quill/pkg/transport/quill"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

func main() {
	log.Println("Starting Quill server...")

	
	serviceAccountKeyPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY_PATH")
	if serviceAccountKeyPath == "" {
		log.Fatalf("Fatal error: FIREBASE_SERVICE_ACCOUNT_KEY_PATH environment variable not set.")
	}

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

	messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

	serverAddr := "localhost:9876"
	server := quill.NewServer(serverAddr, messageHandler)

	log.Printf("INFO: starting Quill protocol server on %s", serverAddr)
	if err := server.Start(); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
}
