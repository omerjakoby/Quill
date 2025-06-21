package models

import (
	"time"
)

// User represents a user in the Quill system
type User struct {
	// UsersUID is the Firebase User ID. It MUST be mapped to MongoDB's _id
	// for efficient lookups and automatic uniqueness enforcement.
	// The `_id,omitempty` tag tells the MongoDB driver to use this field as the _id.
	UsersUID      string    `bson:"_id,omitempty" json:"usersUID"`
	UserQuillMail string    `bson:"userQuillMail" json:"userQuillMail"`
	UserEmail     string    `bson:"userEmail" json:"userEmail"`
	CreatedAt     time.Time `bson:"createdAt" json:"createdAt"` // Corrected "CreatedAt" to "createdAt" for common JSON/BSON convention
	LastLogin     time.Time `bson:"lastLogin" json:"lastLogin"` // Corrected "LastLogin" to "lastLogin"
}

// CreateUserRequest represents the request data for creating a new user
type CreateUserRequest struct {
	AuthToken     string `json:"authToken"` // Removed bson:"authToken" as this isn't saved to DB
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
