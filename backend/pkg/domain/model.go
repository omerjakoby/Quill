// pkg/domain/model.go
package domain

import (
	"time"
)

// ------------------ General Domain Types ------------------

// DomainEmailBody represents plain-text and HTML content of an email.
type DomainEmailBody struct {
	Text string // plain-text body
	HTML string // optional HTML body
}

// DomainAttachment represents a link-only attachment.
type DomainAttachment struct {
	Filename string // file name
	Mimetype string // MIME type
	Link     string // URL or link to attachment content
}

// DomainEmailFlags indicates message status flags.
type DomainEmailFlags struct {
	HasAttachments bool
	IsRead         bool
	IsStarred      bool
}

// ------------------ SEND Request and Result ------------------

// DomainSendRequest represents the data passed from transport to domain to send an email.
type DomainSendRequest struct {
	MessageID   string             // unique ID for this message (client-generated)
	ThreadID    *string            // existing thread, or nil to start a new thread
	From        string             // sender address
	To          []string           // recipients
	CC          []string           // carbon-copy recipients
	BCC         []string           // blind carbon-copy recipients
	Subject     string             // email subject
	Body        DomainEmailBody    // text and HTML content
	Attachments []DomainAttachment // attachments as metadata links
	Options     DomainSendOptions  // send parameters
}

// DomainSendOptions holds optional parameters for sending.
type DomainSendOptions struct {
	ExpiresInSeconds *int  // TTL for message deletion
	OneTime          *bool // if true, message is single-read
}

// DomainSendResult is returned after a SEND_EMAIL operation.
type DomainSendResult struct {
	MessageID   string   // echoed back message ID
	ThreadID    string   // thread into which message was placed
	DeliveredTo []string // actual recipients who received the message
}

// ------------------ FETCH Request ------------------

// FetchMode indicates overview vs thread fetch modes.
type FetchMode string

const (
	FetchModeOverview FetchMode = "overview"
	FetchModeThread   FetchMode = "thread"
)

// DomainFetchRequest represents the criteria for fetching emails.
type DomainFetchRequest struct {
	Mode     FetchMode           // "overview" or "thread"
	Folder   *string             // mailbox name for overview mode
	ThreadID *string             // thread identifier for thread mode
	Limit    *int                // pagination limit
	Offset   *int                // pagination offset
	Filters  *DomainEmailFilters // optional filter criteria
}

// DomainEmailFilters groups possible fetch filters.
type DomainEmailFilters struct {
	Search    *DomainSearchFilter    // keyword and address search
	Flags     *DomainFlagFilter      // read/starred/attachment filters
	DateRange *DomainDateRangeFilter // before/after timestamp filters
}

// DomainSearchFilter filters by keywords, exact phrases, and addresses.
type DomainSearchFilter struct {
	Keywords    []string // keywords to match
	ExactPhrase string   // exact phrase match
	From        []string // sender addresses
	To          []string // recipient addresses
}

// DomainFlagFilter filters by read status, starred status, and attachments presence.
type DomainFlagFilter struct {
	HasAttachments *bool
	IsRead         *bool
	IsStarred      *bool
}

// DomainDateRangeFilter filters by timestamps.
type DomainDateRangeFilter struct {
	After  *time.Time // inclusive
	Before *time.Time // exclusive
}

// ------------------ FETCH Result ------------------

// DomainFetchResult represents the result of a fetch operation.
type DomainFetchResult struct {
	// Common pagination fields
	Limit  int
	Offset int

	// Overview mode results
	TotalThreads    int                    // total number of threads
	ThreadOverviews []DomainThreadOverview // thread summaries

	// Thread mode results
	TotalMessages int             // total messages in thread
	Messages      []DomainMessage // full messages in thread
}

// DomainThreadOverview summarizes the latest message in a thread.
type DomainThreadOverview struct {
	ThreadID      string               // thread identifier
	LatestMessage DomainMessageSummary // summary of most recent message
	Count         int                  // total messages in thread
	UnreadCount   int                  // unread message count
}

// DomainMessageSummary holds brief info about a single message.
type DomainMessageSummary struct {
	ID        string           // message ID
	ThreadID  string           // thread ID
	From      string           // sender
	Subject   string           // message subject
	Timestamp time.Time        // when sent
	Snippet   string           // short excerpt of body
	Flags     DomainEmailFlags // read/starred/attachment flags
}

// DomainMessage is the full message data returned in thread mode.
type DomainMessage struct {
	ID          string             // message ID
	ThreadID    string             // thread ID
	From        string             // sender
	To          []string           // recipients
	CC          []string           // CC
	BCC         []string           // BCC
	Subject     string             // subject
	Body        DomainEmailBody    // text/HTML content
	Attachments []DomainAttachment // attachments metadata
	Timestamp   time.Time          // when sent
	Flags       DomainEmailFlags   // read/starred/attachment flags
}

// ------------------ UPDATE Request and Result ------------------

// DomainUpdateRequest represents changes to apply to messages.
type DomainUpdateRequest struct {
	MessageIDs []string          // messages to update
	Folder     *string           // move to folder
	Category   *string           // e.g. label or category
	Flags      DomainUpdateFlags // flag changes
}

// DomainUpdateFlags holds optional flag updates.
type DomainUpdateFlags struct {
	IsRead    *bool
	IsStarred *bool
	IsDeleted *bool
}

// DomainUpdateResult represents the outcome of an update operation.
type DomainUpdateResult struct {
	Results []DomainUpdateResultItem // per-message results
}

// DomainUpdateResultItem holds the result for one message.
type DomainUpdateResultItem struct {
	MessageID string       // the message ID
	Status    string       // "OK" or "ERROR"
	Error     *DomainError // error details if Status is "ERROR"
}

// DomainError describes an error that occurred on an operation.
type DomainError struct {
	Code    string // error code (e.g. THREAD_NOT_FOUND)
	Message string // human-readable message
}
