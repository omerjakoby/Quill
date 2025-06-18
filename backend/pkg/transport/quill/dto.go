package quill

import (
	"encoding/json"
	"time"
)

// generic packet
type Packet struct {
	Protocol     string          `json:"protocol"`
	Version      string          `json:"version"`
	Type         string          `json:"type"`
	SessionToken string          `json:"session_token,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

// --- Payload definitions for client requests --- //

// SEND
type SendPayload struct {
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	BCC         []string     `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Body        BodyPayload  `json:"body"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Options     SendOptions  `json:"options"`
}

type BodyPayload struct {
	Content []ContentPart `json:"content"`
}

type ContentPart struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Attachment struct {
	Filename      string `json:"filename"`
	Mimetype      string `json:"mimetype"`
	ContentBase64 string `json:"content_base64"`
}

type SendOptions struct {
	ExpiresInSeconds int    `json:"expires_in_seconds,omitempty"`
	OneTime          bool   `json:"one_time,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
}

// FETCH
type FetchPayload struct {
	Mode     string `json:"mode"`
	ThreadID string `json:"thread_id,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// --- Payload definitions for server responses --- //

// generic error reply
type ErrorResponsePayload struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SEND_RESPONSE
type SendResponsePayload struct {
	Status      string   `json:"status"`
	MessageID   string   `json:"message_id"`
	ThreadID    string   `json:"thread_id"`
	DeliveredTo []string `json:"delivered_to,omitempty"`
	QueuedFor   []string `json:"queued_for,omitempty"`
}

// For FETCH_RESPONSE on success.
type FetchResponsePayload struct {
	Status   string       `json:"status"`
	Mode     string       `json:"mode"`
	// Messages []MessageDTO `json:"messages,omitempty"`
	Total    int          `json:"total,omitempty"`
	Limit    int          `json:"limit,omitempty"`
	Offset   int          `json:"offset,omitempty"`
}
