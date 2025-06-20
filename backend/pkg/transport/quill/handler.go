package quill

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"quill/pkg/domain"
	"strings"
	"time"
)

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
	// var err error
	// ctx, err = h.authSvc.Authenticate(ctx, packet.SessionToken)
	// if err != nil {
	// 	log.Printf("WARN: authentication failed for client %s: %v", conn.RemoteAddr(), err)
	// 	h.writeErrorResponse(conn, ErrorCodeAuthFailed, "Invalid or expired session token.")
	// 	return
	// }

	// if userID, ok := UserIDFromContext(ctx); ok {
	// 	log.Printf("INFO: client %s authenticated as user '%s'. Received packet type '%s'",
	// 		conn.RemoteAddr(), userID, packet.Type)
	// } else {
	// 	log.Printf("WARN: Authenticated context missing userID for client %s", conn.RemoteAddr())
	// }

	switch packet.Type {
	case PacketTypeSend:
		h.handleSend(ctx, conn, packet.Payload)
	case PacketTypeFetch:
		h.handleFetch(ctx, conn, packet.Payload)
	case PacketTypePing:
		h.handlePing(conn)
	default:
		log.Printf("WARN: unknown packet type: '%s'", packet.Type)
		h.writeErrorResponse(conn, ErrorCodeUnknownType, "The packet type is not supported.")
	}
}

func (h *MessageHandler) handleSend(ctx context.Context, conn net.Conn, payload json.RawMessage) {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.writeErrorResponse(conn, ErrorCodeInvalidPayload, "Cannot parse SEND payload: "+err.Error())
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
			h.writeErrorResponse(conn, ErrorCodeInvalidContentType, fmt.Sprintf("Invalid content type %q; must be %q or %q", cp.Type, domain.ContentTypePlainText, domain.ContentTypeHTML))
			return
		}
		contents = append(contents, domain.Content{Type: ct, Value: cp.Value})
	}

	// 2) Map attachments
	atts := make([]domain.Attachment, 0, len(req.Attachments))
	for _, a := range req.Attachments {
		atts = append(atts, domain.Attachment{
			Filename: a.Filename,
			Mimetype: a.Mimetype,
			URL:      a.ContentBase64,
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
		MessageID:   req.MessageID,
		From:        req.From,
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
		h.writeErrorResponse(conn, ErrorCodeServiceError, "Failed to send the message.")
		return
	}
	// 6) send packet to non Quill users
	if len(result.QueuedFor) > 0 {
		sendReq := SendPayload{
			MessageID:   result.MessageID,
			From:        req.From,
			To:          req.To,
			CC:          req.CC,
			BCC:         []string{},
			Subject:     req.Subject,
			Body:        req.Body,
			Attachments: req.Attachments,
			Options: SendOptions{
				ExpiresInSeconds: req.Options.ExpiresInSeconds,
				OneTime:          req.Options.OneTime,
				ThreadID:         req.Options.ThreadID,
			},
		}
		for _, addr := range result.QueuedFor {
			if addr == "" {
				continue // skip empty addresses
			}
			addr = extractDomain(addr)
			sendResult, err := sendQuillMessage(addr, sendReq)
			if err != nil {
				log.Printf("ERROR: failed to send message to %s: %v", addr, err)
				h.writeErrorResponse(conn, ErrorCodeDeliveryFailed, fmt.Sprintf("Failed to queue message for %s: %v", addr, err))
				continue
			}
			log.Printf("INFO: queued message %s for external delivery to %s", sendResult, addr)

			log.Printf("INFO: Queued message %s for external delivery to %s", result.MessageID, addr)
		}
	}
	// 7) Construct and send response
	resp := SendResponsePayload{
		Status:      StatusOK,
		MessageID:   result.MessageID,
		ThreadID:    result.ThreadID,
		DeliveredTo: result.DeliveredTo,
		QueuedFor:   result.QueuedFor,
	}
	h.writeResponse(conn, PacketTypeSendResponse, resp)
}

func (h *MessageHandler) handleFetch(ctx context.Context, conn net.Conn, payload json.RawMessage) {
	var req FetchPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		h.writeErrorResponse(conn, ErrorCodeInvalidPayload, "Cannot parse FETCH payload: "+err.Error())
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
		h.writeErrorResponse(conn, ErrorCodeInvalidMode, fmt.Sprintf(
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
		h.writeErrorResponse(conn, ErrorCodeServiceError, "Failed to fetch messages.")
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
			atts[k] = Attachment{Filename: a.Filename, Mimetype: a.Mimetype, ContentBase64: a.URL}
		}

		dtos[i] = MessageDTO{
			MessageID:   m.MessageID,
			ThreadID:    m.ThreadID,
			From:        m.From,
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
		Status:   StatusOK,
		Mode:     req.Mode,
		Messages: dtos,
		Total:    result.Total,
		Limit:    result.Limit,
		Offset:   result.Offset,
	}
	h.writeResponse(conn, PacketTypeFetchResponse, resp)
}

func (h *MessageHandler) handlePing(conn net.Conn) {
	respPayload := PingResponsePayload{
		Status:     StatusOK,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	}
	h.writeResponse(conn, PacketTypePingResponse, respPayload)
}

func (h *MessageHandler) writeResponse(conn net.Conn, packetType string, payload interface{}) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("FATAL: could not marshal response payload: %v", err)
		return
	}

	responsePacket := Packet{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
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
		Status:  StatusError,
		Code:    code,
		Message: message,
	}
	h.writeResponse(conn, PacketTypeErrorResponse, errorPayload)
}

func sendQuillMessage(
	addr string,
	payload SendPayload,
) (*Packet, error) {
	// Marshall the SendPayload into a raw JSON message.
	// This is crucial because our Packet struct expects json.RawMessage for Payload.
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SendPayload: %w", err)
	}

	// Construct the main Quill Packet.
	pkt := Packet{
		Protocol:  "quill",
		Version:   "1.0",
		Type:      "SEND",
		Timestamp: time.Now().UTC(),
		Payload:   json.RawMessage(payloadBytes), // Assign the marshaled bytes
	}

	// Use your existing sendAndReceiveTLS function.
	fmt.Println("Attempting to send Quill message...")
	return sendAndReceiveTLS(addr, &pkt)
}

func sendAndReceiveTLS(addr string, pkt *Packet) (*Packet, error) {
	// 1) Load the self-signed cert so we can trust it
	caPath := "../certificate/quill.crt"
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("could not read CA file %s: %w", caPath, err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to append CA cert")
	}

	// 2) Build a TLS config that trusts that cert
	tlsCfg := &tls.Config{
		RootCAs:            roots,
		ServerName:         "localhost", // must match the CN in quill.crt
		InsecureSkipVerify: true,
	}

	// 3) Dial via TLS instead of plain TCP
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("tls.Dial(%q) failed: %w", addr, err)
	}
	defer conn.Close()

	// 4) Send your Packet as JSON
	if err := json.NewEncoder(conn).Encode(pkt); err != nil {
		return nil, fmt.Errorf("failed to send packet: %w", err)
	}

	// 5) Read and decode the JSON response
	var resp Packet
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed by server")
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}
func extractDomain(input string) string {
	lastTildeIndex := strings.LastIndex(input, "~")
	if lastTildeIndex != -1 {
		return input[lastTildeIndex+1:]
	}
	return ""
}
