package quill

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
)

//TODO add checking for signature
//TODO add checking for anti_spam
//TODO add context Cancellation/timeouts
//TODO add replay protection
//TODO add graceful shutdown

// connectionPhase represents the current stage of the protocol exchange
type connectionPhase int

const (
	phaseAwaitHandshake connectionPhase = iota
	phaseAwaitAuthOrKeys
	phaseReady
)

// ProtocolHandler orchestrates framing, phase transitions, and delegates payloads
type ProtocolHandler struct {
	service *ServiceHandler
}

// NewProtocolHandler initializes a ProtocolHandler with the given service layer
func NewProtocolHandler(svc *ServiceHandler) *ProtocolHandler {
	return &ProtocolHandler{service: svc}
}

type AuthInfoKey struct{}

type AuthInfo struct {
	UserID string
}

// Serve starts processing on the provided connection until closed or fatal error
func (p *ProtocolHandler) Serve(conn net.Conn) {
	defer conn.Close()
	ctx := context.WithValue(context.Background(), AuthInfoKey{}, &AuthInfo{})
	phase := phaseAwaitHandshake

	for {
		pkt, err := p.readPacket(conn)
		if err != nil {
			return // connection closed or framing error
		}

		if pkt.Type == PacketTypePing {
			p.parsePingPayload(ctx, conn, pkt)
			continue
		}

		switch phase {
		case phaseAwaitHandshake:
			if ok := p.handleStateHandshake(ctx, conn, pkt); !ok {
				return // handshake failed
			}
			phase = phaseAwaitAuthOrKeys

		case phaseAwaitAuthOrKeys:
			phase = p.handleStateAuthOrKeys(ctx, conn, pkt)

		case phaseReady:
			if !p.handleStateRequest(ctx, conn, pkt) {
				return // invalid request after auth
			}
		}
	}
}

// readPacket handles length-prefixed framing and unmarshals JSON to Packet
func (p *ProtocolHandler) readPacket(conn net.Conn) (*Packet, error) {
	// 1. Read 4-byte length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// 2. Read JSON body
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		p.sendError(conn, ErrorCodeRequestTimeout, err.Error())
		return nil, err
	}

	// 3. Unmarshal to Packet
	var pkt Packet
	if err := json.Unmarshal(body, &pkt); err != nil {
		p.sendError(conn, ErrorCodeMalformedPacket, err.Error())
		return nil, err
	}
	return &pkt, nil
}

// handleStateHandshake parses and executes the transport-level handshake
// Returns true if handshake succeeded and we should continue
func (p *ProtocolHandler) handleStateHandshake(ctx context.Context, conn net.Conn, pkt *Packet) bool {
	return p.parseHandshakePayload(ctx, conn, pkt)
}

// handleStateAuthOrKeys processes AUTH or FETCH_KEYS and returns next phase
func (p *ProtocolHandler) handleStateAuthOrKeys(ctx context.Context, conn net.Conn, pkt *Packet) connectionPhase {
	switch pkt.Type {
	case PacketTypeAuth:
		if ok := p.parseAuthPayload(ctx, conn, pkt); ok {
			return phaseReady
		}
		return phaseAwaitAuthOrKeys

	case PacketTypeFetchKeys:
		p.parseFetchKeysPayload(ctx, conn, pkt)
		return phaseAwaitAuthOrKeys

	default:
		p.sendError(conn, ErrorCodeMalformedPacket, "expected AUTH or FETCH_KEYS")
		return phaseAwaitAuthOrKeys
	}
}

// handleStateRequest dispatches business requests once authenticated
// Returns false on fatal error
func (p *ProtocolHandler) handleStateRequest(ctx context.Context, conn net.Conn, pkt *Packet) bool {
	switch pkt.Type {
	case PacketTypeSendEmail:
		p.parseSendEmailPayload(ctx, conn, pkt)

	case PacketTypeFetchEmail:
		p.parseFetchEmailsPayload(ctx, conn, pkt)

	case PacketTypeUpdateEmail:
		p.parseUpdateEmailPayload(ctx, conn, pkt)

	default:
		p.sendError(conn, ErrorCodeMalformedPacket, "operation not allowed in current phase")
		return false
	}
	return true
}

// parsePingPayload handles a client PING by echoing its timestamp back in a PING_RESPONSE.
func (p *ProtocolHandler) parsePingPayload(ctx context.Context, conn net.Conn, pkt *Packet) {
	// pkt.Timestamp holds the original client-sent time
	ack := PingAckPayload{
		EchoTimestamp: pkt.Timestamp,
	}

	data, err := json.Marshal(ack)
	if err != nil {
		p.sendError(conn, ErrorCodeInternalServerError, "failed to marshal PingAckPayload")
		return
	}

	p.sendResponse(conn, &Packet{Type: PacketTypePingAck, Payload: data})
}

// parseHandshakePayload unmarshals HANDSHAKE and executes transport handshake
func (p *ProtocolHandler) parseHandshakePayload(ctx context.Context, conn net.Conn, pkt *Packet) bool {
	var payload HandshakePayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeMalformedPacket, "invalid handshake payload: "+err.Error())
		return false
	}
	return p.executeTransportHandshake(ctx, conn, payload)
}

// executeTransportHandshake validates and responds to the handshake
func (p *ProtocolHandler) executeTransportHandshake(ctx context.Context, conn net.Conn, payload HandshakePayload) bool {
	if payload.Protocol != ProtocolName {
		p.sendError(conn, ErrorCodeUnsupportedVersion, "unsupported protocol: "+payload.Protocol)
		return false
	}
	// verify version support
	//TODO change the constant file ProtocolVersion to list and change the logic to pick the highest stable version
	supported := false
	for _, v := range payload.SupportedVersions {
		if v == ProtocolVersion {
			supported = true
			break
		}
	}
	if !supported {
		p.sendError(conn, ErrorCodeUnsupportedVersion, "protocol version not supported")
		return false
	}
	// build and send ACK
	//TODO change the RequiredAntiSpam to not be hardcoded and defined elsewhere
	//TODO change the options to not be hardcoded
	ack := HandshakeAckPayload{
		Accepted: true,
		Version:  ProtocolVersion,
		RequiredAntiSpam: map[string]AntiSpamPolicy{
			PacketTypeSendEmail:  {Type: "hashcash", Bits: 22},
			PacketTypeFetchEmail: {Type: "hashcash", Bits: 22},
			PacketTypeFetchKeys:  {Type: "none"},
		},
		Options: struct {
			Encryption bool `json:"encryption"`
		}{Encryption: payload.Options.Encryption},
	}
	data, _ := json.Marshal(ack)
	p.sendResponse(conn, &Packet{Type: PacketTypeHandshakeAck, Payload: data})
	return true
}

// parseAuthPayload unmarshals AUTH and delegates to service logic
func (p *ProtocolHandler) parseAuthPayload(ctx context.Context, conn net.Conn, pkt *Packet) bool {
	var payload AuthPayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeInvalidPayload, err.Error())
		return false
	}
	// service returns AuthAckPayload and optional ErrorPayload
	ack, errDto := p.service.HandleAuth(ctx, payload)
	if errDto != nil {
		p.sendError(conn, errDto.Code, errDto.Message)
		return false
	}
	data, _ := json.Marshal(ack)
	p.sendResponse(conn, &Packet{Type: PacketTypeAuthAck, Payload: data})
	return true
}

// parseFetchKeysPayload unmarshals FETCH_KEYS and delegates to service
func (p *ProtocolHandler) parseFetchKeysPayload(ctx context.Context, conn net.Conn, pkt *Packet) {
	var payload FetchKeysPayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeInvalidPayload, err.Error())
		return
	}
	resp, errDto := p.service.HandleFetchKeys(ctx, payload)
	if errDto != nil {
		p.sendError(conn, errDto.Code, errDto.Message)
		return
	}
	data, _ := json.Marshal(resp)
	p.sendResponse(conn, &Packet{Type: PacketTypeKeyResponse, Payload: data})
}

// parseSendEmailPayload unmarshals SEND_EMAIL and delegates to service
func (p *ProtocolHandler) parseSendEmailPayload(ctx context.Context, conn net.Conn, pkt *Packet) {
	var payload SendEmailPayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeInvalidPayload, err.Error())
		return
	}
	ack, errDto := p.service.HandleSendEmail(ctx, payload)
	if errDto != nil {
		p.sendError(conn, errDto.Code, errDto.Message)
		return
	}
	data, _ := json.Marshal(ack)
	p.sendResponse(conn, &Packet{Type: PacketTypeSendEmailAck, Payload: data})
}

// parseFetchEmailsPayload unmarshals FETCH_EMAIL and delegates to service
func (p *ProtocolHandler) parseFetchEmailsPayload(ctx context.Context, conn net.Conn, pkt *Packet) {
	var payload FetchEmailPayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeInvalidPayload, err.Error())
		return
	}
	resp, errDto := p.service.HandleFetchEmail(ctx, payload)
	if errDto != nil {
		p.sendError(conn, errDto.Code, errDto.Message)
		return
	}
	data, _ := json.Marshal(resp)
	p.sendResponse(conn, &Packet{Type: PacketTypeFetchEmailResponse, Payload: data})
}

// parseUpdateEmailPayload unmarshals UPDATE_EMAIL and delegates to service
func (p *ProtocolHandler) parseUpdateEmailPayload(ctx context.Context, conn net.Conn, pkt *Packet) {
	var payload UpdateEmailPayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeInvalidPayload, err.Error())
		return
	}
	ack, errDto := p.service.HandleUpdateEmail(ctx, payload)
	if errDto != nil {
		p.sendError(conn, errDto.Code, errDto.Message)
		return
	}
	data, _ := json.Marshal(ack)
	p.sendResponse(conn, &Packet{Type: PacketTypeUpdateEmailAck, Payload: data})
}

// sendResponse marshals a Packet and writes it with framing
func (p *ProtocolHandler) sendResponse(conn net.Conn, pkt *Packet) error {
	//TODO add timestamp
	//TODO add signature
	raw, _ := json.Marshal(pkt)
	buf := make([]byte, 4+len(raw))
	binary.BigEndian.PutUint32(buf, uint32(len(raw)))
	copy(buf[4:], raw)
	_, err := conn.Write(buf)
	return err
}

// sendError constructs and sends an ERROR packet
func (p *ProtocolHandler) sendError(conn net.Conn, code, msg string) {
	//TODO add support for context, temp and retry after
	errPayload := ErrorPayload{Code: code, Message: msg, Context: ""}
	b, _ := json.Marshal(errPayload)
	pkt := &Packet{Type: PacketTypeError, Payload: b}
	_ = p.sendResponse(conn, pkt)
}
