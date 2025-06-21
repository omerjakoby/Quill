package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http" // Import the net/http package
	"os"
	"os/signal" // For graceful shutdown
	"quill/pkg/db"
	"quill/pkg/domain"
	"quill/pkg/models"
	"quill/pkg/transport/quill"
	"strings"
	"syscall" // For graceful shutdown
	"time"
)

func main() {
	log.Println("Starting Quill server...")

	// Create a context that can be cancelled to signal goroutines to stop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called on exit

	// Set up OS signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// --- Existing Quill Server Setup ---
	authSvc, err := quill.InitAuthServiceFromEnv(ctx, "../.env") // Pass context
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}

	mongoURI := getEnvWithDefault("MONGODB_URI", "mongodb://localhost:27017")
	mongoPassword := getEnvWithDefault("mongodb_password", "")
	mongoDatabase := getEnvWithDefault("MONGODB_DATABASE", "quill")

	if mongoPassword != "" && strings.Contains(mongoURI, "<db_password>") {
		mongoURI = strings.Replace(mongoURI, "<db_password>", mongoPassword, 1)
	}

	mongoConfig := db.MongoConfig{
		URI:      mongoURI,
		Database: mongoDatabase,
		Timeout:  10 * time.Second,
	}

	log.Printf("Connecting to MongoDB: %s (database: %s)", maskConnectionString(mongoURI), mongoDatabase)

	mongoDB, err := db.NewMongoDB(mongoConfig)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB successfully")

	msgSvc := domain.NewMongoMessageService(mongoDB.GetDatabase())
	log.Println("Created MongoDB-backed message service")

	messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

	quillServerAddr := "localhost:9876"
	quillServer := quill.NewServer(quillServerAddr, messageHandler)

	// --- Start Quill Server in a Goroutine ---
	go func() {
		log.Printf("INFO: starting Quill protocol TLS server on %s", quillServerAddr)
		// Assuming quill.Server has a StartTLS method. You might want to pass the context
		// to it if you want to cleanly shut it down.
		if err := quillServer.StartTLS("../certificate/quill.crt", "../certificate/quill.key"); err != nil {
			// Don't use log.Fatalf here, as it exits the whole program.
			// Instead, log the error and signal main to shut down.
			log.Printf("FATAL: Quill server failed: %v", err)
			cancel() // Signal main to shut down
		}
	}()

	// --- Add HTTP Server Setup ---
	httpServerAddr := "localhost:8080"
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, Omer! This is the HTTP server speaking from %s\n", r.Host)
	})
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	httpMux.HandleFunc("/createUser", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the JSON request body
		var req models.CreateUserRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Printf("Error decoding request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request fields
		if req.UserQuillMail == "" || req.UserEmail == "" || req.UsersUID == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Create a new User object
		now := time.Now()
		user := &models.User{
			UserQuillMail: req.UserQuillMail,
			UserEmail:     req.UserEmail,
			UsersUID:      req.UsersUID,
			CreatedAt:     now,
			LastLogin:     now,
		}

		// Insert the user into MongoDB
		created, err := mongoDB.CreateUserDoc(r.Context(), user)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Prepare response
		resp := models.CreateUserResponse{
			Success: created,
		}

		if created {
			resp.Message = "User created successfully"
			resp.UserID = user.UsersUID
		} else {
			resp.Message = "User with this email or Quill mail already exists"
			w.WriteHeader(http.StatusConflict) // 409 Conflict
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	httpServer := &http.Server{
		Addr:    httpServerAddr,
		Handler: httpMux,
		// Add timeouts for robustness in production
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// --- Start HTTP Server in a Goroutine ---
	// Inside the goroutine for the HTTP server
	go func() {
		log.Printf("INFO: starting HTTPS server on %s", httpServerAddr)
		// This serves HTTPS (encrypted)
		// You need to provide paths to your TLS certificate and private key files
		certFile := "../certificate/quill.crt" // Reusing your existing certificate path for demonstration
		keyFile := "../certificate/quill.key"  // Reusing your existing key path for demonstration

		if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("FATAL: HTTPS server failed: %v", err)
			cancel()
		}
	}()

	// --- Graceful Shutdown Logic ---
	select {
	case sig := <-sigChan:
		log.Printf("INFO: Received signal %s. Shutting down...", sig)
	case <-ctx.Done():
		log.Println("INFO: A server goroutine signalled shutdown.")
	}

	// Create a shutdown context with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Attempt to gracefully shut down the HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("ERROR: HTTP server shutdown failed: %v", err)
	} else {
		log.Println("INFO: HTTP server shut down gracefully.")
	}

	// For the Quill server, you'd ideally have a `Shutdown` method
	// or a way to close its listener. If quill.Server.StartTLS blocks
	// until an error or close, you might need to add a channel
	// to signal it to close its listener.
	// For now, we'll assume it exits cleanly after an error or will be
	// killed by process exit if it doesn't have a clean shutdown method.
	// A more robust quill.Server would have a context-aware `StartTLS`
	// or `Shutdown` method.

	log.Println("INFO: All servers shut down. Exiting.")
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
