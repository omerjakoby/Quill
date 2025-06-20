package db

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
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
