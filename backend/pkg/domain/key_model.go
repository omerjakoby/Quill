package domain

import "time"

// FetchKeysRequest is what your transport layer hands the service.
type FetchKeysRequest struct {
	Query string // user identity or domain
}

// FetchKeysResult holds a key entries.
type FetchKeysResult struct {
	Identity  string    // e.g. "alice~quillmail.com"
	PublicKey string    // base64 or PEM-encoded
	Expires   time.Time // expiration
}
