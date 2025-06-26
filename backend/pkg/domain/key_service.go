package domain

import (
	"context"
	"time"
)

// MockKeyService is a mock implementation of the KeyService interface.
type MockKeyService struct{}

// NewMockKeyService creates a new MockKeyService.
func NewMockKeyService() *MockKeyService {
	return &MockKeyService{}
}

// FetchKeys returns mock key data.
func (m *MockKeyService) FetchKeys(ctx context.Context, req FetchKeysRequest) (FetchKeysResult, error) {
	return FetchKeysResult{
		Identity:  req.Query,
		PublicKey: "mock-public-key",
		Expires:   time.Now().Add(24 * time.Hour),
	}, nil
}
