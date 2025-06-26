package quill

import "encoding/json"

// Packet is the top-level structure for all Quill protocol messages.
type Packet struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature *string         `json:"signature,omitempty"`
	AntiSpam  *AntiSpamProof  `json:"anti_spam,omitempty"`
}

// AntiSpamProof carries the proof-of-work header negotiated during handshake.
type AntiSpamProof struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Bits     int    `json:"bits"`
	Nonce    string `json:"nonce"`
}

// -------------------- Handshake --------------------

// HandshakePayload is sent by the client to initiate protocol negotiation.
type HandshakePayload struct {
	Protocol          string   `json:"protocol"`
	SupportedVersions []string `json:"supported_versions"`
	Identity          string   `json:"identity"`
	Options           struct {
		Encryption bool `json:"encryption"`
	} `json:"options"`
	SupportedAntiSpam []string `json:"supported_anti_spam"`
}

// HandshakeAckPayload is sent by the server to confirm negotiation parameters.
type HandshakeAckPayload struct {
	Accepted         bool                      `json:"accepted"`
	Version          string                    `json:"version"`
	RequiredAntiSpam map[string]AntiSpamPolicy `json:"required_anti_spam"`
	Options          struct {
		Encryption bool `json:"encryption"`
	} `json:"options"`
}

// AntiSpamPolicy describes proof-of-work parameters per operation.
type AntiSpamPolicy struct {
	Type string `json:"type"`
	Bits int    `json:"bits"`
}

// -------------------- Authentication --------------------

// Credentials carries a token for AuthPayload.
type Credentials struct {
	Token string `json:"token"`
}

// AuthPayload carries authentication parameters from the client.
type AuthPayload struct {
	Method      string      `json:"method"`
	Credentials Credentials `json:"credentials"`
}

// SessionInfo returns details in AuthAckPayload.
type SessionInfo struct {
	ExpiresIn int    `json:"expires_in"`
	Identity  string `json:"identity"`
}

// AuthAckPayload acknowledges authentication.
type AuthAckPayload struct {
	Accepted bool        `json:"accepted"`
	Session  SessionInfo `json:"session"`
}

// -------------------- Sending Email --------------------

// EmailBody represents the body of an email.
type EmailBody struct {
	Text string `json:"text"`
	HTML string `json:"html,omitempty"`
}

// Attachment describes a link-only attachment.
type Attachment struct {
	Filename string `json:"filename"`
	Mimetype string `json:"mimetype"`
	Link     string `json:"link"`
}

// SendOptions configures email send parameters.
type SendOptions struct {
	ExpiresInSeconds int  `json:"expires_in_seconds,omitempty"`
	OneTime          bool `json:"one_time,omitempty"`
}

// SendEmailPayload is used for SEND_EMAIL packets.
type SendEmailPayload struct {
	MessageID   string       `json:"message_id"`
	ThreadID    string       `json:"thread_id"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc,omitempty"`
	Bcc         []string     `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Body        EmailBody    `json:"body"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Options     SendOptions  `json:"options,omitempty"`
}

// SendEmailAckPayload is used for SEND_EMAIL_ACK packets.
type SendEmailAckPayload struct {
	Status      string   `json:"status"`
	MessageID   string   `json:"message_id,omitempty"`
	DeliveredTo []string `json:"delivered_to,omitempty"`
}

// -------------------- Key Discovery --------------------

// FetchKeysPayload is used for FETCH_KEYS packets.
type FetchKeysPayload struct {
	Query string `json:"query"`
}

// KeyResponsePayload is used for KEY_RESPONSE packets.
type KeyResponsePayload struct {
	Email     string `json:"email"`
	PublicKey string `json:"public_key"`
	Expires   string `json:"expires"`
}

// -------------------- Fetching Email --------------------

// SearchFilter supports keyword and address filtering.
type SearchFilter struct {
	Keywords    []string `json:"keywords,omitempty"`
	ExactPhrase string   `json:"exact_phrase,omitempty"`
	From        []string `json:"from,omitempty"`
	To          []string `json:"to,omitempty"`
}

// FlagFilter supports filtering by flags.
type FlagFilter struct {
	HasAttachments *bool `json:"has_attachments,omitempty"`
	IsRead         *bool `json:"is_read,omitempty"`
	IsStarred      *bool `json:"is_starred,omitempty"`
}

// DateRangeFilter supports after/before filtering.
type DateRangeFilter struct {
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
}

// EmailFilters groups filter criteria.
type EmailFilters struct {
	Search    *SearchFilter    `json:"search,omitempty"`
	Flags     *FlagFilter      `json:"flags,omitempty"`
	DateRange *DateRangeFilter `json:"date_range,omitempty"`
}

// FetchEmailPayload is used for FETCH_EMAIL packets.
type FetchEmailPayload struct {
	Mode     string        `json:"mode"`
	Folder   string        `json:"folder,omitempty"`
	ThreadID string        `json:"thread_id,omitempty"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	Filters  *EmailFilters `json:"filters,omitempty"`
}

// MessageSummary describes brief info for overview.
type MessageSummary struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	From      string `json:"from"`
	Subject   string `json:"subject"`
	Timestamp string `json:"timestamp"`
	Snippet   string `json:"snippet"`
	Flags     struct {
		HasAttachments bool `json:"has_attachments"`
		IsStarred      bool `json:"is_starred"`
	} `json:"flags"`
}

// ThreadOverview describes thread entries in overview mode.
type ThreadOverview struct {
	ThreadID      string         `json:"thread_id"`
	LatestMessage MessageSummary `json:"latest_message"`
	Count         int            `json:"count"`
	UnreadCount   int            `json:"unread_count"`
}

// MessageBody is the full body in thread mode.
type MessageBody struct {
	Text string `json:"text"`
	HTML string `json:"html,omitempty"`
}

// MessageAttachment describes attachments in thread mode.
type MessageAttachment struct {
	Filename string `json:"filename"`
	Mimetype string `json:"mimetype"`
	Link     string `json:"link"`
}

// MessageDTO is used for FETCH_EMAIL_RESPONSE in thread mode.
type MessageDTO struct {
	ID          string              `json:"id"`
	From        string              `json:"from"`
	To          []string            `json:"to"`
	Cc          []string            `json:"cc,omitempty"`
	Bcc         []string            `json:"bcc,omitempty"`
	Subject     string              `json:"subject"`
	Body        MessageBody         `json:"body"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	Timestamp   string              `json:"timestamp"`
	Flags       struct {
		HasAttachments bool `json:"has_attachments"`
		IsRead         bool `json:"is_read"`
		IsStarred      bool `json:"is_starred"`
	} `json:"flags"`
}

// FetchEmailResponsePayload is used for FETCH_EMAIL_RESPONSE packets.
type FetchEmailResponsePayload struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`

	// overview mode
	Threads      []ThreadOverview `json:"threads,omitempty"`
	TotalThreads int              `json:"total_threads,omitempty"`

	// thread mode
	ThreadID      string       `json:"thread_id,omitempty"`
	Messages      []MessageDTO `json:"messages,omitempty"`
	TotalMessages int          `json:"total_messages,omitempty"`

	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// -------------------- Updating Email --------------------

// UpdateFlags holds flag updates.
type UpdateFlags struct {
	IsRead    *bool `json:"is_read,omitempty"`
	IsStarred *bool `json:"is_starred,omitempty"`
	IsDeleted *bool `json:"is_deleted,omitempty"`
}

// UpdateEmailPayload is used for UPDATE_EMAIL packets.
type UpdateEmailPayload struct {
	MessageIDs []string `json:"message_ids"`
	Updates    struct {
		Folder   string      `json:"folder,omitempty"`
		Category string      `json:"category,omitempty"`
		Flags    UpdateFlags `json:"flags,omitempty"`
	} `json:"updates"`
}

// UpdateResultItem represents one update result.
type UpdateResultItem struct {
	MessageID string        `json:"message_id"`
	Status    string        `json:"status"`
	Error     *ErrorPayload `json:"error,omitempty"`
}

// UpdateEmailAckPayload is used for UPDATE_EMAIL_ACK packets.
type UpdateEmailAckPayload struct {
	Results []UpdateResultItem `json:"results"`
}

// -------------------- Ping --------------------

// -------------------- Errors --------------------

// ErrorPayload is used for ERROR packets.
type ErrorPayload struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Context    string `json:"context,omitempty"`
	Temporary  *bool  `json:"temporary,omitempty"`
	RetryAfter *int   `json:"retry_after,omitempty"`
}
