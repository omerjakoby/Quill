package domain

import "time"

// DomainFetchKeysRequest is what your transport layer hands the service.
type DomainFetchKeysRequest struct {
    Query string // user identity or domain
}

// DomainKeyEntry is one public-key record.
type DomainKeyEntry struct {
    Identity  string    // e.g. "alice~quillmail.com"
    PublicKey string    // base64 or PEM-encoded
    Expires   time.Time // expiration
}

// DomainFetchKeysResult holds zero or more key entries.
type DomainFetchKeysResult struct {
    Keys []DomainKeyEntry
}
