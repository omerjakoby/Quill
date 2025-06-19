package domain

import (
	"time"
)

// ----- SEND Request and Result -----

// Attachment represents an attachment *after* it's been uploaded to GCS.
// It will be sent by the client and stored directly in the DB.
type Attachment struct {
	Filename string `bson:"filename"` // For MongoDB persistence
	Mimetype string `bson:"mimetype"` // For MongoDB persistence
	URL      string `bson:"url"`      // The URL to the GCS object, for MongoDB persistence
}

// DomainSendRequest is the structure of the incoming email data.
// Attachments now directly include the GCS URL.
type DomainSendRequest struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        Body
	Attachments []Attachment // Directly uses the 'Attachment' struct with GCS URL
	Options     SendOptions
}

// Body holds one or more content parts.
type Body struct {
	Content []Content
}

// Content is a piece of the email body.
type Content struct {
	Type  ContentType
	Value string
}

type ContentType string

const (
	ContentTypePlainText ContentType = "text/plain"
	ContentTypeHTML      ContentType = "text/html"
)

// SendOptions provides additional parameters for sending.
type SendOptions struct {
	ExpiresInSeconds *int
	OneTime          *bool
	ThreadID         *string
}

// DomainSendResult is what's returned after a successful send operation.
type DomainSendResult struct {
	MessageID   string
	ThreadID    string
	DeliveredTo []string
	QueuedFor   []string
}

// ----- FETCH Request and Result -----

type FetchMode string

const (
	FetchModeThread FetchMode = "thread"
	FetchModeFolder FetchMode = "folder"
)

// DomainFetchRequest specifies how messages should be fetched.
type DomainFetchRequest struct {
	Mode     FetchMode
	ThreadID *string
	Folder   *string
	Limit    *int
	Offset   *int
}

// DomainFetchResult contains the fetched messages and pagination info.
type DomainFetchResult struct {
	Total    int
	Limit    int
	Offset   int
	Messages []Message
}

// Message is the full email structure returned during a fetch operation.
type Message struct {
	MessageID   string
	ThreadID    string
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        Body
	Attachments []Attachment // Still uses the Attachment struct with the URL
	SentAt      time.Time
	Read        bool
	Flags       []string
}
