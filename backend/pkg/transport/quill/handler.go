package quill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// --- Service Interfaces ---
// These define the contract between the transport layer and the business logic layer.

type authService interface {
	// Authenticates a token and returns a user context (e.g., userID).
	Authenticate(ctx context.Context, token string) (context.Context, error)
}

type messageService interface {
	// The service layer works with Domain objects, not transport DTOs.
	//Send(ctx context.Context, req DomainSendRequest) (*DomainSendResult, error)
	//Fetch(ctx context.Context, threadID string) ([]MessageDTO, error) // For simplicity, let's say fetch returns DTOs directly
}

// MessageHandler handles the protocol logic for a single TCP connection.
type MessageHandler struct {
	authSvc    authService
	messageSvc messageService
}

// NewMessageHandler creates a new handler with its required dependencies.
func NewMessageHandler(as authService, ms messageService) *MessageHandler {
	return &MessageHandler{
		authSvc:    as,
		messageSvc: ms,
	}
}

// Handle is the entry point for a new connection. It reads and dispatches packets.
func (h *MessageHandler) Handle(conn net.Conn) {
	defer conn.Close()
	log.Printf("INFO: new client connected: %s", conn.RemoteAddr())

	decoder := json.NewDecoder(conn)
	for {
		var packet Packet
		if err := decoder.Decode(&packet); err != nil {
			if err == io.EOF {
				log.Printf("INFO: client disconnected cleanly: %s", conn.RemoteAddr())
			} else {
				log.Printf("ERROR: could not decode packet from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		// Dispatch the packet to the correct handler.
		h.dispatch(conn, &packet)
	}
}

// dispatch validates the packet and routes it to the correct specific handler.
func (h *MessageHandler) dispatch(conn net.Conn, packet *Packet) {
	// --- Authentication ---
	// Create a base context for this request.
	ctx := context.Background() // Start with an empty base context for this request.

	// Authenticate the session token. The Authenticate method will return
	// a *new* context with the user ID embedded if successful, or an error.
	var err error // Declare err here for reassignment
	ctx, err = h.authSvc.Authenticate(ctx, packet.SessionToken)
	if err != nil {
		log.Printf("WARN: authentication failed for client %s: %v", conn.RemoteAddr(), err)
		h.writeErrorResponse(conn, "AUTH_FAILED", "Invalid or expired session token.")
		return // Stop processing this packet if authentication fails
	}

	// Now, 'ctx' contains the authenticated user's ID.
	// You can even log it to verify:
	if userID, ok := UserIDFromContext(ctx); ok {
		log.Printf("INFO: client %s authenticated as user '%s'. Received packet type '%s'",
			conn.RemoteAddr(), userID, packet.Type)
	} else {
		// This case should ideally not happen if Authenticate was successful,
		// but it's good practice for safety.
		log.Printf("WARN: Authenticated context missing userID for client %s", conn.RemoteAddr())
	}

	switch packet.Type {
	case "SEND":
		// Pass the *authenticated* context down to the handler
		h.handleSend(ctx, conn, packet.Payload)
	case "FETCH":
		// Pass the *authenticated* context down to the handler
		h.handleFetch(ctx, conn, packet.Payload)
	default:
		log.Printf("WARN: unknown packet type: '%s'", packet.Type)
		h.writeErrorResponse(conn, "UNKNOWN_TYPE", "The packet type is not supported.")
	}
}

// handleSend processes a "SEND" packet.
func (h *MessageHandler) handleSend(ctx context.Context, conn net.Conn, payload json.RawMessage) {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.writeErrorResponse(conn, "INVALID_PAYLOAD", "Cannot parse SEND payload: "+err.Error())
		return
	}

	// --- DTO to Domain Model Conversion ---
	// domainReq := DomainSendRequest{
	// 	To:      req.To,
	// 	Subject: req.Subject,
	// 	Body:    req.Body.Content[0].Value, // Simplified for example
	// }

	// --- Call Business Logic ---
	//result, err := h.messageSvc.Send(ctx, domainReq)
	// if err != nil {
	// 	// Here you could check for specific business errors from the service
	// 	// and return different error codes.
	// 	log.Printf("ERROR: service call to Send failed: %v", err)
	// 	h.writeErrorResponse(conn, "SERVICE_ERROR", "Failed to send the message.")
	// 	return
	// }

	// --- Create and Send Response ---
	// respPayload := SendResponsePayload{
	// 	Status:    "OK",
	// 	MessageID: result.MessageID,
	// 	ThreadID:  result.ThreadID,
	// }
	// h.writeResponse(conn, "SEND_RESPONSE", respPayload)
}

// handleFetch processes a "FETCH" packet.
func (h *MessageHandler) handleFetch(ctx context.Context, conn net.Conn, payload json.RawMessage) {
	var req FetchPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.writeErrorResponse(conn, "INVALID_PAYLOAD", "Cannot parse FETCH payload: "+err.Error())
		return
	}

	// --- Call Business Logic ---
	// messages, err := h.messageSvc.Fetch(ctx, req.ThreadID)
	// if err != nil {
	// 	log.Printf("ERROR: service call to Fetch failed: %v", err)
	// 	h.writeErrorResponse(conn, "SERVICE_ERROR", "Failed to fetch messages.")
	// 	return
	// }

	// --- Create and Send Response ---
	// respPayload := FetchResponsePayload{
	// 	Status:   "OK",
	// 	Mode:     req.Mode,
	// 	Messages: messages,
	// 	Total:    len(messages),
	// }
	// h.writeResponse(conn, "FETCH_RESPONSE", respPayload)
}

// --- Helper Functions ---

// writeResponse is a generic helper to construct and send any successful response packet.
func (h *MessageHandler) writeResponse(conn net.Conn, packetType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("FATAL: could not marshal response payload: %v", err)
		// Don't send an error back, because the server itself is broken.
		return
	}

	responsePacket := Packet{
		Protocol:  "quill",
		Version:   "1.0",
		Type:      packetType,
		Timestamp: time.Now().UTC(),
		Payload:   payloadBytes,
	}

	if err := json.NewEncoder(conn).Encode(responsePacket); err != nil {
		log.Printf("ERROR: failed to write response to client %s: %v", conn.RemoteAddr(), err)
	}
}

// writeErrorResponse is a convenience helper for sending standardized error packets.
func (h *MessageHandler) writeErrorResponse(conn net.Conn, code, message string) {
	errorPayload := ErrorResponsePayload{
		Status:  "ERROR",
		Code:    code,
		Message: message,
	}
	h.writeResponse(conn, "ERROR_RESPONSE", errorPayload)
}

type SimpleAuthService struct {
	// map of valid sessionToken → userID
	tokens map[string]string
}

// constructor for your authService
func NewSimpleAuthService(tokens map[string]string) authService {
	return &SimpleAuthService{tokens: tokens}
}

// Authenticate checks the token against the in-memory map,
// and if valid, returns a new context carrying the userID.
func (s *SimpleAuthService) Authenticate(
	ctx context.Context,
	token string,
) (context.Context, error) {
	userID, ok := s.tokens[token]
	if !ok {
		return ctx, fmt.Errorf("invalid or expired token")
	}
	// attach the userID to the context for downstream handlers
	return context.WithValue(ctx, userKey, userID), nil
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userKey)
	id, ok := v.(string)
	return id, ok
}
