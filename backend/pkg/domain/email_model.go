// pkg/domain/model.go
package domain

import (
	"time"
)

// ------------------ General Domain Types ------------------

// EmailBody represents plain-text and HTML content of an email.
type EmailBody struct {
	Text string // plain-text body
	HTML string // optional HTML body
}

// Attachment represents a link-only attachment.
type Attachment struct {
	Filename string // file name
	Mimetype string // MIME type
	Link     string // URL or link to attachment content
}

// EmailFlags indicates message status flags.
type EmailFlags struct {
	HasAttachments bool
	IsRead         bool
	IsStarred      bool
}

// ------------------ SEND Request and Result ------------------

// SendEmailRequest represents the data passed from transport to domain to send an email.
type SendEmailRequest struct {
	MessageID   string           // unique ID for this message (client-generated)
	ThreadID    *string          // existing thread, or nil to start a new thread
	From        string           // sender address
	To          []string         // recipients
	CC          []string         // carbon-copy recipients
	BCC         []string         // blind carbon-copy recipients
	Subject     string           // email subject
	Body        EmailBody        // text and HTML content
	Attachments []Attachment     // attachments as metadata links
	Options     SendEmailOptions // send parameters
}

// SendEmailOptions holds optional parameters for sending.
type SendEmailOptions struct {
	ExpiresInSeconds *int  // TTL for message deletion
	OneTime          *bool // if true, message is single-read
}

// SendEmailResult is returned after a SEND_EMAIL operation.
type SendEmailResult struct {
	MessageID   string   // echoed back message ID
	ThreadID    string   // thread into which message was placed
	DeliveredTo []string // actual recipients who received the message
}

// ------------------ FETCH Request ------------------

// FetchMode indicates overview vs thread fetch modes.
type FetchEmailMode string

const (
	FetchModeOverview FetchEmailMode = "overview"
	FetchModeThread   FetchEmailMode = "thread"
)

// FetchRequest represents the criteria for fetching emails.
type FetchEmailRequest struct {
	Mode     FetchEmailMode     // "overview" or "thread"
	Folder   *string            // mailbox name for overview mode
	ThreadID *string            // thread identifier for thread mode
	Limit    *int               // pagination limit
	Offset   *int               // pagination offset
	Filters  *FetchEmailFilters // optional filter criteria
}

// FetchEmailFilters groups possible fetch filters.
type FetchEmailFilters struct {
	Search    *SearchFilter    // keyword and address search
	Flags     *FlagFilter      // read/starred/attachment filters
	DateRange *DateRangeFilter // before/after timestamp filters
}

// SearchFilter filters by keywords, exact phrases, and addresses.
type SearchFilter struct {
	Keywords    []string // keywords to match
	ExactPhrase string   // exact phrase match
	From        []string // sender addresses
	To          []string // recipient addresses
}

// FlagFilter filters by read status, starred status, and attachments presence.
type FlagFilter struct {
	HasAttachments *bool
	IsRead         *bool
	IsStarred      *bool
}

// DateRangeFilter filters by timestamps.
type DateRangeFilter struct {
	After  *time.Time // inclusive
	Before *time.Time // exclusive
}

// ------------------ FETCH Result ------------------

// FetchResult represents the result of a fetch operation.
type FetchEmailResult struct {
	// Common pagination fields
	Limit  int
	Offset int

	// Overview mode results
	TotalThreads    int              // total number of threads
	ThreadOverviews []ThreadOverview // thread summaries

	// Thread mode results
	TotalMessages int       // total messages in thread
	Messages      []Message // full messages in thread
}

// ThreadOverview summarizes the latest message in a thread.
type ThreadOverview struct {
	ThreadID      string         // thread identifier
	LatestMessage MessageSummary // summary of most recent message
	Count         int            // total messages in thread
	UnreadCount   int            // unread message count
}

// MessageSummary holds brief info about a single message.
type MessageSummary struct {
	ID        string     // message ID
	ThreadID  string     // thread ID
	From      string     // sender
	Subject   string     // message subject
	Timestamp time.Time  // when sent
	Snippet   string     // short excerpt of body
	Flags     EmailFlags // read/starred/attachment flags
}

// Message is the full message data returned in thread mode.
type Message struct {
	ID          string       // message ID
	ThreadID    string       // thread ID
	From        string       // sender
	To          []string     // recipients
	CC          []string     // CC
	BCC         []string     // BCC
	Subject     string       // subject
	Body        EmailBody    // text/HTML content
	Attachments []Attachment // attachments metadata
	Timestamp   time.Time    // when sent
	Flags       EmailFlags   // read/starred/attachment flags
}

// ------------------ UPDATE Request and Result ------------------

// UpdateRequest represents changes to apply to messages.
type UpdateEmailRequest struct {
	MessageIDs []string         // messages to update
	Folder     *string          // move to folder
	Category   *string          // e.g. label or category
	Flags      UpdateEmailFlags // flag changes
}

// UpdateEmailFlags holds optional flag updates.
type UpdateEmailFlags struct {
	IsRead    *bool
	IsStarred *bool
	IsDeleted *bool
}

// UpdateEmailResult represents the outcome of an update operation.
type UpdateEmailResult struct {
	Results []UpdateEmailResultItem // per-message results
}

// UpdateEmailResultItem holds the result for one message.
type UpdateEmailResultItem struct {
	MessageID string      // the message ID
	Status    string      // "OK" or "ERROR"
	Error     *EmailError // error details if Status is "ERROR"
}

// Error describes an error that occurred on an operation.
type EmailError struct {
	Code    string // error code (e.g. THREAD_NOT_FOUND)
	Message string // human-readable message
}
