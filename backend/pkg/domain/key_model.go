package domain

import "time"

// FetchKeysRequest is what your transport layer hands the service.
type FetchKeysRequest struct {
	Query string // user identity or domain
}

// FetchKeyEntry is one public-key record.
type FetchKeyEntry struct {
	Identity  string    // e.g. "alice~quillmail.com"
	PublicKey string    // base64 or PEM-encoded
	Expires   time.Time // expiration
}

// FetchKeysResult holds zero or more key entries.
type FetchKeysResult struct {
	Keys []FetchKeyEntry
}
