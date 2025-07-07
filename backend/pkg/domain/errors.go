package domain

// Custom error types for the domain layer.

// DomainError represents a custom error with a code and message.
// This allows for more specific error handling in the transport layer.
type DomainError struct {
	Code    string
	Message string
	Inner   error
}

func (e *DomainError) Error() string {
	if e.Inner != nil {
		return e.Message + ": " + e.Inner.Error()
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Inner
}

// NewDomainError creates a new domain error with the specified code and message
func NewDomainError(code, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}

// NewDomainErrorWithCause creates a new domain error with the specified code, message, and underlying cause
func NewDomainErrorWithCause(code, message string, inner error) *DomainError {
	return &DomainError{Code: code, Message: message, Inner: inner}
}

// Domain error codes that map to transport layer error codes
const (
	// Authentication & Authorization
	ErrCodeAuthRequired     = "AUTH_REQUIRED"
	ErrCodeInvalidToken     = "INVALID_TOKEN"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeSenderMismatch   = "SENDER_MISMATCH"

	// Validation errors
	ErrCodeInvalidPayload    = "INVALID_PAYLOAD"
	ErrCodeInvalidRecipients = "INVALID_RECIPIENTS"
	ErrCodeInvalidFilter     = "INVALID_FILTER"
	ErrCodeInvalidMode       = "INVALID_MODE"

	// Resource errors
	ErrCodeUserNotFound   = "USER_NOT_FOUND"
	ErrCodeFolderNotFound = "FOLDER_NOT_FOUND"
	ErrCodeThreadNotFound = "THREAD_NOT_FOUND"

	// Message errors
	ErrCodeTooLarge             = "TOO_LARGE"
	ErrCodeBlockedDomain        = "BLOCKED_DOMAIN"
	ErrCodeRecipientUnavailable = "RECIPIENT_UNAVAILABLE"

	// Internal errors (fallback)
	ErrCodeInternalError = "INTERNAL_SERVER_ERROR"
)

// Legacy error constants for backward compatibility
type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	// ErrAuth represents an authentication error.
	ErrAuth Error = "AUTHENTICATION_ERROR"
	// ErrPermission represents a permission error.
	ErrPermission Error = "PERMISSION_ERROR"
	// ErrValidation represents a validation error for invalid input.
	ErrValidation Error = "VALIDATION_ERROR"
	// ErrNotFound represents a resource not found error.
	ErrNotFound Error = "NOT_FOUND_ERROR"
)
