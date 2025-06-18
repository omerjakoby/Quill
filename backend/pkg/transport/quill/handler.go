package quill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"quill/pkg/domain"
	"strings"
	"time"

	"firebase.google.com/go/auth"
)

type userContextKey struct{}

var userKey = userContextKey{}

// --- Service Interfaces ---
type authService interface {
	Authenticate(ctx context.Context, token string) (context.Context, error)
}
type messageService interface {
	// The service layer works with Domain objects, not transport DTOs.
	Send(ctx context.Context, req domain.DomainSendRequest) (domain.DomainSendResult, error)
	Fetch(ctx context.Context, req domain.DomainFetchRequest) (domain.DomainFetchResult, error)
}
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

	// The `packet.SessionToken` is the Firebase ID Token.
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

	// 1) DTO → Domain: map and validate content parts
	contents := make([]domain.Content, 0, len(req.Body.Content))
	for _, cp := range req.Body.Content {
		ct := domain.ContentType(cp.Type)
		switch ct {
		case domain.ContentTypePlainText, domain.ContentTypeHTML:
			// valid
		default:
			h.writeErrorResponse(conn, "INVALID_CONTENT_TYPE", fmt.Sprintf("Invalid content type %q; must be %q or %q", cp.Type, domain.ContentTypePlainText, domain.ContentTypeHTML))
			return
		}
		contents = append(contents, domain.Content{Type: ct, Value: cp.Value})
	}

	// 2) Map attachments
	atts := make([]domain.Attachment, 0, len(req.Attachments))
	for _, a := range req.Attachments {
		atts = append(atts, domain.Attachment{
			Filename:      a.Filename,
			Mimetype:      a.Mimetype,
			ContentBase64: a.ContentBase64,
		})
	}

	// 3) Optional fields → pointers
	var expiresPtr *int
	if req.Options.ExpiresInSeconds > 0 {
		expiresPtr = &req.Options.ExpiresInSeconds
	}

	var oneTimePtr *bool
	if req.Options.OneTime {
		oneTimePtr = &req.Options.OneTime
	}

	var threadIDPtr *string
	if req.Options.ThreadID != "" {
		threadIDPtr = &req.Options.ThreadID
	}

	// 4) Build domain request
	domainReq := domain.DomainSendRequest{
		To:          req.To,
		CC:          req.CC,
		BCC:         req.BCC,
		Subject:     req.Subject,
		Body:        domain.Body{Content: contents},
		Attachments: atts,
		Options:     domain.SendOptions{ExpiresInSeconds: expiresPtr, OneTime: oneTimePtr, ThreadID: threadIDPtr},
	}

	// 5) Call service
	result, err := h.messageSvc.Send(ctx, domainReq)
	if err != nil {
		log.Printf("ERROR: service call to Send failed: %v", err)
		h.writeErrorResponse(conn, "SERVICE_ERROR", "Failed to send the message.")
		return
	}

	// 6) Construct and send response
	resp := SendResponsePayload{
		Status:      "OK",
		MessageID:   result.MessageID,
		ThreadID:    result.ThreadID,
		DeliveredTo: result.DeliveredTo,
		QueuedFor:   result.QueuedFor,
	}
	h.writeResponse(conn, "SEND_RESPONSE", resp)
}

func (h *MessageHandler) handleFetch(ctx context.Context, conn net.Conn, payload json.RawMessage) {
	var req FetchPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.writeErrorResponse(conn, "INVALID_PAYLOAD", "Cannot parse FETCH payload: "+err.Error())
		return
	}

	// 1) DTO → Domain: map & validate fetch parameters
	var mode domain.FetchMode
	switch req.Mode {
	case string(domain.FetchModeThread):
		mode = domain.FetchModeThread
	case string(domain.FetchModeFolder):
		mode = domain.FetchModeFolder
	default:
		h.writeErrorResponse(conn, "INVALID_MODE", fmt.Sprintf(
			"Invalid fetch mode %q; must be %q or %q", req.Mode,
			string(domain.FetchModeThread), string(domain.FetchModeFolder),
		))
		return
	}

	var threadIDPtr *string
	if req.ThreadID != "" {
		threadIDPtr = &req.ThreadID
	}
	var folderPtr *string
	if req.Folder != "" {
		folderPtr = &req.Folder
	}
	var limitPtr *int
	if req.Limit > 0 {
		limitPtr = &req.Limit
	}
	var offsetPtr *int
	if req.Offset > 0 {
		offsetPtr = &req.Offset
	}

	// 2) Build domain request
	domainReq := domain.DomainFetchRequest{
		Mode:     mode,
		ThreadID: threadIDPtr,
		Folder:   folderPtr,
		Limit:    limitPtr,
		Offset:   offsetPtr,
	}

	// 3) Call business layer
	result, err := h.messageSvc.Fetch(ctx, domainReq)
	if err != nil {
		log.Printf("ERROR: service call to Fetch failed: %v", err)
		h.writeErrorResponse(conn, "SERVICE_ERROR", "Failed to fetch messages.")
		return
	}

	// 4) Map domain messages to DTOs
	dtos := make([]MessageDTO, len(result.Messages))
	for i, m := range result.Messages {
		// map body parts
		bp := BodyPayload{Content: make([]ContentPart, len(m.Body.Content))}
		for j, c := range m.Body.Content {
			bp.Content[j] = ContentPart{Type: string(c.Type), Value: c.Value}
		}

		// map attachments
		atts := make([]Attachment, len(m.Attachments))
		for k, a := range m.Attachments {
			atts[k] = Attachment{Filename: a.Filename, Mimetype: a.Mimetype, ContentBase64: a.ContentBase64}
		}

		dtos[i] = MessageDTO{
			MessageID:   m.MessageID,
			ThreadID:    m.ThreadID,
			From:        strings.Join(m.From, ","),
			To:          m.To,
			CC:          m.CC,
			BCC:         m.BCC,
			Subject:     m.Subject,
			Body:        bp,
			Attachments: atts,
			SentAt:      m.SentAt,
			Read:        m.Read,
			Flags:       m.Flags,
		}
	}

	// 5) Construct and send response
	resp := FetchResponsePayload{
		Status:   "OK",
		Mode:     req.Mode,
		Messages: dtos,
		Total:    result.Total,
		Limit:    result.Limit,
		Offset:   result.Offset,
	}
	h.writeResponse(conn, "FETCH_RESPONSE", resp)
}

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

// --- Firebase Authentication Service ---
type FirebaseAuthService struct {
	firebaseAuthClient *auth.Client
}

func NewFirebaseAuthService(client *auth.Client) authService {
	return &FirebaseAuthService{
		firebaseAuthClient: client,
	}
}

func (s *FirebaseAuthService) Authenticate(
	ctx context.Context,
	idToken string,
) (context.Context, error) {
	idToken = strings.TrimPrefix(idToken, "Bearer ")
	token, err := s.firebaseAuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify Firebase ID token: %w", err)
	}

	// Token is valid. Attach the Firebase User ID (UID) to the context.
	return context.WithValue(ctx, userKey, token.UID), nil
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userKey)
	id, ok := v.(string)
	return id, ok
}
