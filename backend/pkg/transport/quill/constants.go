package quill

// ——————————————————————————————
// Protocol Handshake settings
// ——————————————————————————————

// Supported protocol versions (in order of stability)
var SupportedProtocolVersions = []int{1}

// DefaultProtocolVersion is the highest stable version clients will negotiate.
const DefaultProtocolVersion = 1

// AntiSpamPolicy tells you which anti-spam scheme & parameters to enforce per packet type.
var DefaultRequiredAntiSpam = map[string]AntiSpamPolicy{
	PacketTypeSendEmail:  {Type: "hashcash", Bits: 20},
	PacketTypeFetchEmail: {Type: "hashcash", Bits: 18},
	PacketTypeFetchKeys:  {Type: "none"},
}

// Which features we offer at handshake time.
var DefaultHandshakeOptions = struct {
	Encryption bool
}{
	Encryption: false,
}

// ——————————————————————————————
// Session settings
// ——————————————————————————————

const (
	// DefaultSessionExpiresIn is how long (in seconds) new sessions last if caller
	DefaultSessionExpiresIn = 3600
)

// ——————————————————————————————
// Quill Protocol Const
// ——————————————————————————————

// Protocol identification
const (
	ProtocolName    = "quill"
	ProtocolVersion = "1.0"
)

// Packet types
const (
	PacketTypeHandshake          = "HANDSHAKE"
	PacketTypeHandshakeAck       = "HANDSHAKE_ACK"
	PacketTypeAuth               = "AUTH"
	PacketTypeAuthAck            = "AUTH_ACK"
	PacketTypeSendEmail          = "SEND_EMAIL"
	PacketTypeSendEmailAck       = "SEND_EMAIL_ACK"
	PacketTypeFetchKeys          = "FETCH_KEYS"
	PacketTypeKeyResponse        = "KEY_RESPONSE"
	PacketTypeFetchEmail         = "FETCH_EMAIL"
	PacketTypeFetchEmailResponse = "FETCH_EMAIL_RESPONSE"
	PacketTypeUpdateEmail        = "UPDATE_EMAIL"
	PacketTypeUpdateEmailAck     = "UPDATE_EMAIL_ACK"
	PacketTypePing               = "PING"
	PacketTypePingAck            = "PING_ACK"
	PacketTypeError              = "ERROR"
)

// Status values for ACK payloads
const (
	StatusOK    = "OK"
	StatusError = "ERROR"
)

// Standardized error codes for the Quill protocol
const (
	// 1. General & Protocol Errors
	ErrorCodeMalformedPacket     = "MALFORMED_PACKET"
	ErrorCodeInvalidPayload      = "INVALID_PAYLOAD"
	ErrorCodeInvalidTimestamp    = "INVALID_TIMESTAMP"
	ErrorCodeRateLimited         = "RATE_LIMITED"
	ErrorCodeTooManyConnections  = "TOO_MANY_CONNECTIONS"
	ErrorCodeRequestTimeout      = "REQUEST_TIMEOUT"
	ErrorCodeInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrorCodeProtocolDisabled    = "PROTOCOL_DISABLED"

	// 2. Handshake Errors
	ErrorCodeUnsupportedVersion = "UNSUPPORTED_VERSION"
	ErrorCodeTLSRequired        = "TLS_REQUIRED"
	ErrorCodeIdentityMismatch   = "IDENTITY_MISMATCH"
	ErrorCodeSpamPolicyMismatch = "SPAM_POLICY_MISMATCH"

	// 3. Authentication & Authorization Errors
	ErrorCodeAuthRequired        = "AUTH_REQUIRED"
	ErrorCodeUnsupportedMethod   = "UNSUPPORTED_METHOD"
	ErrorCodeInvalidToken        = "INVALID_TOKEN"
	ErrorCodeCredentialsRejected = "CREDENTIALS_REJECTED"
	ErrorCodeTooManyAttempts     = "TOO_MANY_ATTEMPTS"
	ErrorCodeChallengeFailed     = "CHALLENGE_FAILED"
	ErrorCodeSenderMismatch      = "SENDER_MISMATCH"
	ErrorCodePermissionDenied    = "PERMISSION_DENIED"

	// 4. Message & Data Transfer Errors
	ErrorCodeInvalidSignature     = "INVALID_SIGNATURE"
	ErrorCodeSignatureRequired    = "SIGNATURE_REQUIRED"
	ErrorCodeInvalidRecipients    = "INVALID_RECIPIENTS"
	ErrorCodeRecipientUnavailable = "RECIPIENT_UNAVAILABLE"
	ErrorCodeTooLarge             = "TOO_LARGE"
	ErrorCodeBlockedDomain        = "BLOCKED_DOMAIN"
	ErrorCodeFolderNotFound       = "FOLDER_NOT_FOUND"
	ErrorCodeThreadNotFound       = "THREAD_NOT_FOUND"
	ErrorCodeInvalidFilter        = "INVALID_FILTER"
	ErrorCodeInvalidMode          = "INVALID_MODE"

	// 5. Anti-Spam Errors
	ErrorCodeSpamProofRequired = "SPAM_PROOF_REQUIRED"
	ErrorCodeInvalidSpamProof  = "INVALID_SPAM_PROOF"
	ErrorCodeSpamDetected      = "SPAM_DETECTED"

	// 6. Federation & Key Management Errors
	ErrorCodeUserNotFound     = "USER_NOT_FOUND"
	ErrorCodeFederationDenied = "FEDERATION_DENIED"
)
