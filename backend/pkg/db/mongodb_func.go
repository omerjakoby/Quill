package db

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"quill/pkg/models"
)

// MessageIDExists checks if a message with the given ID already exists in the messages collection.
func (m *MongoDB) MessageIDExists(ctx context.Context, messageID string) (bool, error) {
	collection := m.GetMessagesCollection()
	filter := bson.M{"messageId": messageID}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateUserDoc creates a new user document in the users collection.
// It returns true if the user was created successfully, false if a user with the same email already exists.
func (m *MongoDB) CreateUserDoc(ctx context.Context, user *models.User) (bool, error) {
	collection := m.GetUsersCollection()

	// Check if user already exists
	filter := bson.M{"$or": []bson.M{
		{"userEmail": user.UserEmail},
		{"userQuillMail": user.UserQuillMail},
	}}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("error checking for existing user: %w", err)
	}

	if count > 0 {
		return false, nil // User already exists
	}

	// Insert the user document
	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		return false, fmt.Errorf("error inserting user document: %w", err)
	}

	return true, nil
}
