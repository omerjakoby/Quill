package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http" // Import the net/http package
	"os"
	"os/signal" // For graceful shutdown
	"strings"
	"syscall" // For graceful shutdown
	"time"

	"github.com/joho/godotenv"

	"quill/cmd/main/constants"
	"quill/pkg/db"
	"quill/pkg/domain"
	"quill/pkg/models"
	"quill/pkg/service/auth"
	"quill/pkg/transport/quill"
)

// main is the entry point for the Quill server application, initializing services, starting protocol and HTTP servers, and handling graceful shutdown on termination signals.
func main() {
	log.Println("Starting Quill server...")

	// Create a context that can be cancelled to signal goroutines to stop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called on exit

	// Set up OS signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize services and databases
	//TODO OMER read the todo in line 48 and fix
	authSvc, mongoDB, emailSvc, keySvc := initializeServices()

	// Configure and start servers
	quillServer := setupQuillServer(ctx, cancel, authSvc, emailSvc, keySvc)
	httpServer := setupHTTPServer(ctx, cancel, mongoDB, authSvc)

	// Wait for shutdown signal
	waitForShutdown(ctx, sigChan, httpServer, quillServer)
}

//TODO OMER: create the struct and implement for emailSvc and keySvc then init them here
// initializeServices initializes and returns the authentication service, MongoDB connection, and placeholders for email and key services.
func initializeServices() (auth.AuthService, *db.MongoDB, ??new emailSvc type, ???new keySvc type) {
	// Auth Service initialization
	authSvcCtx, authSvcCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer authSvcCancel()

	authSvc, err := auth.InitAuthServiceFromEnv(authSvcCtx, "../.env")
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}
	
	// MongoDB initialization
	mongoDB := initializeMongoDB()

	// Message service initialization
	msgSvc := domain.NewMongoMessageService(mongoDB.GetDatabase())
	log.Println("Created MongoDB-backed message service")

	return authSvc, mongoDB, emailSvc, keySvc
}

// initializeMongoDB establishes a connection to MongoDB using environment variables and ensures required indexes are created.
// It returns the connected MongoDB instance. The function logs fatal errors and terminates the program if the connection or index creation fails.
func initializeMongoDB() *db.MongoDB {
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

	// Create indexes
	ensureMongoDBIndexes(mongoDB)

	return mongoDB
}

// ensureMongoDBIndexes creates required unique indexes on user collections in MongoDB.
// Terminates the application if index creation fails.
func ensureMongoDBIndexes(mongoDB *db.MongoDB) {
	log.Println("Ensuring MongoDB unique user indexes...")
	indexCtx, indexCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer indexCancel()

	if err := mongoDB.EnsureUniqueUserIndexes(indexCtx); err != nil {
		log.Fatalf("Failed to ensure unique user indexes: %v", err)
	}
	log.Println("MongoDB unique user indexes ensured successfully.")
}

// setupQuillServer initializes and launches the Quill protocol server with TLS support.
// The server is started asynchronously and will signal cancellation if startup fails.
// Returns the Quill server instance.
func setupQuillServer(ctx context.Context, cancel context.CancelFunc, authSvc quill.AuthService, emailSvc ???, keySvc ???) *quill.Server {
	ServiceHandler := quill.NewServiceHandler(authSvc, emailSvc, keySvc)
	protocolHandler := quill.NewProtocolHandler(ServiceHandler)

	quillServer := quill.NewServer(constants.QuillServerAddr, protocolHandler)

	// Start Quill Server in a goroutine
	go func() {
		log.Printf("INFO: starting Quill protocol TLS server on %s", constants.QuillServerAddr)
		if err := quillServer.StartTLS("../certificate/quill.crt", "../certificate/quill.key"); err != nil {
			log.Printf("FATAL: Quill server failed: %v", err)
			cancel() // Signal main to shut down
		}
	}()

	return quillServer
}

// setupHTTPServer configures and launches the HTTPS server with registered handlers and production-ready timeouts.
// The server is started asynchronously and returns the configured *http.Server instance.
func setupHTTPServer(ctx context.Context, cancel context.CancelFunc, mongoDB *db.MongoDB, authSvc quill.AuthService) *http.Server {
	httpServerAddr := "localhost:8080"
	httpMux := http.NewServeMux()

	// Register handlers
	registerHTTPHandlers(httpMux, mongoDB, authSvc)

	httpServer := &http.Server{
		Addr:    httpServerAddr,
		Handler: httpMux,
		// Add timeouts for robustness in production
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start HTTP Server in a goroutine
	go func() {
		log.Printf("INFO: starting HTTPS server on %s", httpServerAddr)
		certFile := "../certificate/quill.crt"
		keyFile := "../certificate/quill.key"

		if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("FATAL: HTTPS server failed: %v", err)
			cancel()
		}
	}()

	return httpServer
}

// registerHTTPHandlers registers HTTP endpoint handlers for the homepage, health check, and user creation API on the provided ServeMux.
func registerHTTPHandlers(mux *http.ServeMux, mongoDB *db.MongoDB, authSvc quill.AuthService) {
	// Basic homepage handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, Omer! This is the HTTP server speaking from %s\n", r.Host)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// User creation endpoint
	mux.HandleFunc("/createUser", handleCreateUser(mongoDB, authSvc))
}

// handleCreateUser returns an HTTP handler for the /createUser endpoint that processes user creation requests.
// The handler accepts POST requests with a JSON body containing user details, validates required fields,
// creates a new user in MongoDB using the provided authentication service, and responds with a JSON result
// indicating success, conflict if the user already exists, or an error for invalid input or server issues.
func handleCreateUser(mongoDB *db.MongoDB, authSvc quill.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("[/createUser] Received request")
		// Only allow POST method
		if r.Method != http.MethodPost {
			log.Printf("[/createUser] Invalid method: %s", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the JSON request body
		var req models.CreateUserRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Printf("[/createUser] Error decoding request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		log.Printf("[/createUser] Request body: %+v", req)

		// Validate request fields
		if req.UserQuillMail == "" || req.UserEmail == "" || req.UsersUID == "" {
			log.Printf("[/createUser] Missing required fields: UserQuillMail=%q, UserEmail=%q, UsersUID=%q", req.UserQuillMail, req.UserEmail, req.UsersUID)
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
		log.Printf("[/createUser] Creating user: %+v", user)

		// Insert the user into MongoDB
		created, err := mongoDB.CreateUserDoc(r.Context(), user, req.AuthToken, authSvc)
		if err != nil {
			log.Printf("[/createUser] Error creating user: %v", err)
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
			log.Printf("[/createUser] User created successfully: UID=%s, Email=%s, QuillMail=%s", user.UsersUID, user.UserEmail, user.UserQuillMail)
		} else {
			// This branch is hit if mongo.IsDuplicateKeyError(err) was true in CreateUserDoc
			resp.Message = "User with this UID, email, or Quill mail already exists"
			w.WriteHeader(http.StatusConflict) // 409 Conflict
			log.Printf("[/createUser] User already exists: UID=%s, Email=%s, QuillMail=%s", user.UsersUID, user.UserEmail, user.UserQuillMail)
		}

		// Send JSON response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("[/createUser] Error encoding response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		log.Printf("[/createUser] Response sent: %+v", resp)
	}
}

// waitForShutdown waits for a shutdown signal or context cancellation and gracefully shuts down the HTTP server.
// Logs shutdown events and coordinates server termination. The Quill server is not shut down unless a shutdown method is implemented.
func waitForShutdown(ctx context.Context, sigChan chan os.Signal, httpServer *http.Server, quillServer *quill.Server) {
	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("INFO: Received signal %s. Shutting down...", sig)
	case <-ctx.Done():
		log.Println("INFO: A server goroutine signalled shutdown.")
	}

	// Gracefully shut down the HTTP server
	shutdownHTTPServer(httpServer)

	// For the Quill server, ideally you'd also have a shutdown method
	// This part would be implemented if quillServer has a Shutdown method

	log.Println("INFO: All servers shut down. Exiting.")
}

// shutdownHTTPServer attempts to gracefully stop the HTTP server within a 5-second timeout, logging the outcome.
func shutdownHTTPServer(httpServer *http.Server) {
	// Create a shutdown context with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Attempt to gracefully shut down the HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("ERROR: HTTP server shutdown failed: %v", err)
	} else {
		log.Println("INFO: HTTP server shut down gracefully.")
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
