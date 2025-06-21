package db

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"quill/pkg/models"
	"quill/pkg/transport/quill"
)

// EnsureUniqueUserIndexes sets up unique indexes for userQuillMail
// in the users collection. This function should be called once during application
// startup or database initialization.
//
// Note: userEmail uniqueness is guaranteed by Firebase when using "Sign in with Google",
// so no separate MongoDB unique index is needed for that field.
func (m *MongoDB) EnsureUniqueUserIndexes(ctx context.Context) error {
	collection := m.GetUsersCollection()

	// Unique index for UserQuillMail
	quillMailIndexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "userQuillMail", Value: 1}}, // Ascending index
		Options: options.Index().SetUnique(true),
	}
	_, err := collection.Indexes().CreateOne(ctx, quillMailIndexModel)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			fmt.Printf("Warning: Could not create unique index on userQuillMail due to existing duplicates. Please clean data. Error: %v\n", err)
		} else {
			return fmt.Errorf("failed to create unique index on userQuillMail: %w", err)
		}
	} else {
		fmt.Println("Unique index on userQuillMail ensured.")
	}

	// No unique index needed for userEmail because:
	// 1. UsersUID (Firebase UID) is used as _id, enforcing unique Firebase users.
	// 2. Firebase Sign-in with Google guarantees uniqueness of the email for a given Firebase account.
	//    If an email is associated with a Firebase account, that account has a unique UsersUID.
	//    Therefore, the combination of UsersUID and UserEmail is already unique.

	return nil
}

func (m *MongoDB) MessageIDExists(ctx context.Context, messageID string) (bool, error) {
	collection := m.GetMessagesCollection()
	filter := bson.M{"messageId": messageID}

	// A find operation is often more efficient than CountDocuments for simple existence checks,
	// especially if 'messageId' is indexed.
	var result struct{} // We don't care about the content, just existence
	err := collection.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil // Document does not exist
		}
		return false, fmt.Errorf("error checking for message existence: %w", err)
	}

	return true, nil // Document exists
}

// CreateUserDoc creates a new user document in the users collection.
// It leverages MongoDB's unique indexes for UsersUID (_id) and UserQuillMail.
// It returns true if the user was created successfully, false if a user with
// the same UsersUID or UserQuillMail already exists.
// The provided authToken must be a valid Firebase ID token and match the user's UID.
func (m *MongoDB) CreateUserDoc(ctx context.Context, user *models.User, authToken string, authSvc quill.AuthService) (bool, error) {
	// Verify the auth token
	authCtx, err := authSvc.Authenticate(ctx, authToken)
	if err != nil {
		return false, fmt.Errorf("error authenticating token: %w", err)
	}

	// Extract the user ID from the authenticated context
	userID, ok := quill.UserIDFromContext(authCtx)
	if !ok {
		return false, fmt.Errorf("no user ID found in authenticated context")
	}

	// Verify that the token's user ID matches the provided user's UID.
	// This is a crucial security and authorization check.
	if userID != user.UsersUID {
		return false, fmt.Errorf("token user ID '%s' does not match provided user ID '%s'", userID, user.UsersUID)
	}

	collection := m.GetUsersCollection()

	// Attempt to insert the user document.
	// MongoDB will automatically enforce uniqueness for:
	// 1. `_id` (mapped from `user.UsersUID` in your `models.User` struct)
	// 2. `userQuillMail` (due to the unique index we've configured)
	// UserEmail uniqueness is handled by Firebase directly via the Firebase UID.
	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		// Check if the error is due to a duplicate key violation.
		// This handles duplicates for _id or userQuillMail.
		if mongo.IsDuplicateKeyError(err) {
			// A duplicate key error means a user with that _id or Quill mail already exists.
			// Log the specific error for debugging but return false (user already exists).
			fmt.Printf("User creation failed: Duplicate key error. User already exists based on UID or QuillMail. Error: %v\n", err)
			return false, nil // User already exists
		}
		// Handle any other types of insertion errors.
		return false, fmt.Errorf("error inserting user document: %w", err)
	}

	// If no error, the user document was successfully inserted.
	return true, nil
}

// GetQuillMailByUserID retrieves the userQuillMail address for a user by their document ID (UsersUID).
func (m *MongoDB) GetQuillMailByUserID(ctx context.Context, userID string) (string, error) {
	collection := m.GetUsersCollection()
	filter := bson.M{"_id": userID}
	var result struct {
		UserQuillMail string `bson:"userQuillMail"`
	}
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil // User not found
		}
		return "", fmt.Errorf("error retrieving userQuillMail: %w", err)
	}
	return result.UserQuillMail, nil
}
