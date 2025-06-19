package domain

import (
	"context"
	"log"
)

type MockMessageService struct{}

func (m *MockMessageService) Send(ctx context.Context, req DomainSendRequest) (DomainSendResult, error) {
	log.Println("Mock: Sending message to", req.To)
	return DomainSendResult{
		MessageID:   "mock-msg-123",
		ThreadID:    "mock-thread-456",
		DeliveredTo: req.To,
		QueuedFor:   []string{},
	}, nil
}

func (m *MockMessageService) Fetch(ctx context.Context, req DomainFetchRequest) (DomainFetchResult, error) {
	log.Println("Mock: Fetching messages with mode", req.Mode)
	return DomainFetchResult{
		Total:    0,
		Limit:    10,
		Offset:   0,
		Messages: []Message{},
	}, nil
}
