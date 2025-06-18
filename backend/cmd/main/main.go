package main

import (
	"context"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/joho/godotenv"
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
	err := godotenv.Load("C:\\Users\\assij\\GolandProjects\\Quill\\.env") // Assuming main.go is in backend/cmd/main/
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	serviceAccountKeyPath := os.Getenv("firebase_service_account_path")
	if serviceAccountKeyPath == "" {
		log.Fatal("firebase_service_account_path environment variable not set.")
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

	// --- Step 2: Instantiate your FirebaseAuthService ---
	// This service now wraps the Firebase Auth client.
	authService := quill.NewFirebaseAuthService(firebaseAuthClient)
	log.Println("FirebaseAuthService initialized successfully.")
	log.Println(authService)
	//auth service goas in to the auth service

}
