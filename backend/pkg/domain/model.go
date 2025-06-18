package domain

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
