package domain

import (
	"quill/pkg/models"
)

// This file now contains domain-specific user functionality
// The core User type definitions have been moved to pkg/models/user.go

// UserService defines the interface for user-related operations
type UserService interface {
	CreateUser(request models.CreateUserRequest) (models.CreateUserResponse, error)
	// Add other user operations as needed
}
