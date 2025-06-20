// pkg/transport/quill/constants.go
package quill

const (
	// Protocol identification
	ProtocolName    = "quill"
	ProtocolVersion = "1.0"

	// Packet types
	PacketTypeSend          = "SEND"
	PacketTypeFetch         = "FETCH"
	PacketTypeSendResponse  = "SEND_RESPONSE"
	PacketTypeFetchResponse = "FETCH_RESPONSE"
	PacketTypeErrorResponse = "ERROR_RESPONSE"
	PacketTypePing          = "PING"
	PacketTypePingResponse  = "PING_RESPONSE"

	// Error-response payload “status”
	StatusOK    = "OK"
	StatusError = "ERROR"

	// Error codes (used in writeErrorResponse)
	ErrorCodeAuthFailed         = "AUTH_FAILED"
	ErrorCodeUnknownType        = "UNKNOWN_TYPE"
	ErrorCodeInvalidPayload     = "INVALID_PAYLOAD"
	ErrorCodeInvalidContentType = "INVALID_CONTENT_TYPE"
	ErrorCodeServiceError       = "SERVICE_ERROR"
	ErrorCodeInvalidMode        = "INVALID_MODE"
	ErrorCodeDeliveryFailed     = "DELIVERY_FAILED"
	ErrInvalidDomain            = "INVALID_DOMAIN"
)
