package main

import (
	"context"
	"log"
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

	authSvc, err := quill.InitAuthServiceFromEnv(context.Background(), "../../../.env")
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}

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
