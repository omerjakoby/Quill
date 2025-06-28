// in quill/pkg/domain/context.go
package domain

import "context"

// AuthInfoKey is the context key for AuthInfo. It's an empty struct
// to prevent collisions.
type AuthInfoKey struct{}

// AuthInfo holds authentication data passed through the context.
type AuthInfo struct {
	UserID string
}

// UserIDFromContext safely retrieves the UserID from the context.
// It returns the UserID and true if found, otherwise an empty string and false.
func UserIDFromContext(ctx context.Context) (string, bool) {
	// Use the correct key type (domain.AuthInfoKey) to get the value.
	authInfo, ok := ctx.Value(AuthInfoKey{}).(*AuthInfo)

	// Check if the type assertion was successful and the pointer is not nil.
	if !ok || authInfo == nil {
		return "", false
	}

	// Return the UserID field from the struct.
	return authInfo.UserID, true
}
