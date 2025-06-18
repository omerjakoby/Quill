package quill

import (
	"context"
	"encoding/json"
	"firebase.google.com/go/auth" // This is the crucial import for Firebase Auth
	"fmt"
	"io"
	"log"
	"net"
	"strings" // Added for parsing Authorization header
	"time"
)

// --- Domain Models (Placeholders) ---
type userContextKey struct{}

var userKey = userContextKey{}

type DomainSendRequest struct {
	To      []string
	Subject string
	Body    string
}

type DomainSendResult struct {
	MessageID string
	ThreadID  string
}

// --- Service Interfaces ---
type authService interface {
	Authenticate(ctx context.Context, token string) (context.Context, error)
}

type messageService interface {
	// The service layer works with Domain objects, not transport DTOs.
	//Send(ctx context.Context, req DomainSendRequest) (*DomainSendResult, error)
	//Fetch(ctx context.Context, threadID string) ([]MessageDTO, error) // For simplicity, let's say fetch returns DTOs directly
}

// MessageHandler (no changes needed here, as it depends on the interface)
type MessageHandler struct {
	authSvc    authService
	messageSvc messageService
}

func NewMessageHandler(as authService, ms messageService) *MessageHandler {
	return &MessageHandler{
		authSvc:    as,
		messageSvc: ms,
	}
}

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

		h.dispatch(conn, &packet)
	}
}

// dispatch validates the packet and routes it to the correct specific handler.
func (h *MessageHandler) dispatch(conn net.Conn, packet *Packet) {
	ctx := context.Background()

	// The `packet.SessionToken` is now expected to be the Firebase ID Token.
	var err error
	ctx, err = h.authSvc.Authenticate(ctx, packet.SessionToken)
	if err != nil {
		log.Printf("WARN: authentication failed for client %s: %v", conn.RemoteAddr(), err)
		h.writeErrorResponse(conn, "AUTH_FAILED", "Invalid or expired session token.")
		return
	}

	if userID, ok := UserIDFromContext(ctx); ok {
		log.Printf("INFO: client %s authenticated as user '%s'. Received packet type '%s'",
			conn.RemoteAddr(), userID, packet.Type)
	} else {
		log.Printf("WARN: Authenticated context missing userID for client %s", conn.RemoteAddr())
	}

	switch packet.Type {
	case "SEND":
		h.handleSend(ctx, conn, packet.Payload)
	case "FETCH":
		h.handleFetch(ctx, conn, packet.Payload)
	default:
		log.Printf("WARN: unknown packet type: '%s'", packet.Type)
		h.writeErrorResponse(conn, "UNKNOWN_TYPE", "The packet type is not supported.")
	}
}

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
func (h *MessageHandler) writeResponse(conn net.Conn, packetType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("FATAL: could not marshal response payload: %v", err)
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

func (h *MessageHandler) writeErrorResponse(conn net.Conn, code, message string) {
	errorPayload := ErrorResponsePayload{
		Status:  "ERROR",
		Code:    code,
		Message: message,
	}
	h.writeResponse(conn, "ERROR_RESPONSE", errorPayload)
}

// --- NEW Firebase Authentication Service ---

// FirebaseAuthService implements the authService interface using Firebase Admin SDK.
type FirebaseAuthService struct {
	firebaseAuthClient *auth.Client
	// Optional: If you need to check token revocation, you'd store the firebase.App here too,
	// or create a separate VerifyIDTokenAndCheckRevoked method.
}

// NewFirebaseAuthService creates a new FirebaseAuthService instance.
// It requires an initialized Firebase Auth client.
func NewFirebaseAuthService(client *auth.Client) authService {
	return &FirebaseAuthService{
		firebaseAuthClient: client,
	}
}

// Authenticate verifies the Firebase ID Token.
// It returns a new context with the user's UID attached if verification is successful.
func (s *FirebaseAuthService) Authenticate(
	ctx context.Context,
	idToken string,
) (context.Context, error) {
	// Trim "Bearer " prefix if present, although your current client-side sends raw token.
	// For robustness, it's good practice.
	idToken = strings.TrimPrefix(idToken, "Bearer ")

	// Use the Firebase Admin SDK to verify the ID token.
	// This performs all necessary checks: signature, expiration, issuer, audience.
	token, err := s.firebaseAuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify Firebase ID token: %w", err)
	}

	// Token is valid. Attach the Firebase User ID (UID) to the context.
	// The UID is a unique identifier for the user within Firebase.
	return context.WithValue(ctx, userKey, token.UID), nil
}

// UserIDFromContext remains the same, as it extracts from the generic userKey.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userKey)
	id, ok := v.(string)
	return id, ok
}
