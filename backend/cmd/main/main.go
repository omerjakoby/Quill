package main

import (
	"context"
	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
	"log"
	"os"
	"quill/pkg/transport/quill"
)

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
	serviceAccountKeyPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY_PATH")
	if serviceAccountKeyPath == "" {
		log.Fatalf("Fatal error: FIREBASE_SERVICE_ACCOUNT_KEY_PATH environment variable not set.")
		// For local development, you might set a default here or require a flag:
		// serviceAccountKeyPath = "path/to/your-service-account-key.json"
		// log.Println("WARNING: Using default service account key path for development. Do not use in production.")
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

	// --- Step 2: Instantiate your FirebaseAuthService ---
	// This service now wraps the Firebase Auth client.
	authService := quill.NewFirebaseAuthService(firebaseAuthClient)
	//auth service goas in to the auth service

}
