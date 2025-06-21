package models

import (
	"time"
)

// User represents a user in the Quill system
type User struct {
	UserQuillMail string    `bson:"userQuillMail" json:"userQuillMail"`
	UserEmail     string    `bson:"userEmail" json:"userEmail"`
	UsersUID      string    `bson:"usersUID" json:"usersUID"`
	CreatedAt     time.Time `bson:"CreatedAt" json:"createdAt"`
	LastLogin     time.Time `bson:"lastLogin" json:"lastLogin"`
}

// CreateUserRequest represents the request data for creating a new user
type CreateUserRequest struct {
	UserQuillMail string `json:"userQuillMail"`
	UserEmail     string `json:"userEmail"`
	UsersUID      string `json:"usersUID"`
}

// CreateUserResponse represents the response after creating a user
type CreateUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"userId,omitempty"`
}
