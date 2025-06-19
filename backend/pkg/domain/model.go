package domain

import (
	"time"
)

// ----- SEND Request and Result -----
type DomainSendRequest struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        Body
	Attachments []Attachment
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

type Attachment struct {
	Filename      string
	Mimetype      string
	ContentBase64 string
}

type SendOptions struct {
	ExpiresInSeconds *int
	OneTime          *bool
	ThreadID         *string
}

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

type DomainFetchRequest struct {
	Mode     FetchMode
	ThreadID *string
	Folder   *string
	Limit    *int
	Offset   *int
}

type DomainFetchResult struct {
	Total    int
	Limit    int
	Offset   int
	Messages []Message
}

type Message struct {
	MessageID   string
	ThreadID    string
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        Body
	Attachments []Attachment
	SentAt      time.Time
	Read        bool
	Flags       []string
}
