package main

import (
	"context"
	"log"
	"quill/pkg/domain"
	"quill/pkg/transport/quill"
)

func main() {
	log.Println("Starting Quill server...")
	authSvc, err := quill.InitAuthServiceFromEnv(context.Background(), "../.env")
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}

	// Create the mock message service
	msgSvc := &domain.MockMessageService{}
	log.Println("Created mock message service for testing")

	messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

	serverAddr := "localhost:9876"
	server := quill.NewServer(serverAddr, messageHandler)

	log.Printf("INFO: starting Quill protocol server on %s", serverAddr)
	if err := server.Start(); err != nil {
		log.Fatalf("FATAL: could not start server: %v", err)
	}
}
