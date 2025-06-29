package quill

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"quill/pkg/domain"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/umahmood/hashcash"
)

//TODO add checking for signature
//TODO add checking for anti_spam
//TODO add context Cancellation/timeouts
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

// Serve starts processing on the provided connection until closed or fatal error
func (p *ProtocolHandler) Serve(conn net.Conn) {
	defer conn.Close()
	ctx := context.WithValue(context.Background(), domain.AuthInfoKey{}, &domain.AuthInfo{})
	phase := phaseAwaitHandshake

	for {
		pkt, err := p.readPacket(conn)
		if err != nil {
			return // connection closed or framing error
		}

		// relay protection: enforce timestamp skew
		if !p.validateTimestamp(pkt.Timestamp, conn) {
			return
		}

		if pkt.Type == PacketTypePing {
			p.parsePingPayload(conn, pkt)
			continue
		}
		// validate the anti_spam
		// if !p.validateAntiSpam(pkt, conn) {
		// 	return
		// }

		switch phase {
		case phaseAwaitHandshake:
			if ok := p.handleStateHandshake(conn, pkt); !ok {
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

// handleStateHandshake parses and executes the transport-level handshake
// Returns true if handshake succeeded and we should continue
func (p *ProtocolHandler) handleStateHandshake(conn net.Conn, pkt *Packet) bool {
	return p.parseHandshakePayload(conn, pkt)
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
func (p *ProtocolHandler) parsePingPayload(conn net.Conn, pkt *Packet) {
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
func (p *ProtocolHandler) parseHandshakePayload(conn net.Conn, pkt *Packet) bool {
	var payload HandshakePayload
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		p.sendError(conn, ErrorCodeMalformedPacket, "invalid handshake payload: "+err.Error())
		return false
	}
	return p.executeTransportHandshake(conn, payload)
}

// executeTransportHandshake validates and responds to the handshake
func (p *ProtocolHandler) executeTransportHandshake(conn net.Conn, payload HandshakePayload) bool {
	// Protocol name check
	if payload.Protocol != ProtocolName {
		p.sendError(conn, ErrorCodeUnsupportedVersion, "unsupported protocol: "+payload.Protocol)
		return false
	}

	// Version negotiation, picking the highest version
	var negotiatedVersion string
	var highestNum float64

	for _, vs := range payload.SupportedVersions {
		// parse "1.0", "2.1", etc.
		num, err := strconv.ParseFloat(vs, 64)
		if err != nil {
			continue
		}
		major := int(num)

		// check if we support that major version
		supported := slices.Contains(SupportedProtocolVersions, major)
		if !supported {
			continue
		}

		// keep the highest one
		if negotiatedVersion == "" || num > highestNum {
			negotiatedVersion = vs
			highestNum = num
		}
	}

	if negotiatedVersion == "" {
		p.sendError(conn, ErrorCodeUnsupportedVersion,
			"protocol version not supported: "+strings.Join(payload.SupportedVersions, ", "))
		return false
	}

	// build and send ACK
	ack := HandshakeAckPayload{
		Accepted:         true,
		Version:          negotiatedVersion,
		RequiredAntiSpam: DefaultRequiredAntiSpam,
		Options: struct {
			Encryption bool `json:"encryption"`
		}{Encryption: payload.Options.Encryption && DefaultHandshakeOptions.Encryption},
	}
	data, err := json.Marshal(ack)
	if err != nil {
		p.sendError(conn, ErrorCodeInternalServerError, "failed to marshal handshake ack: "+err.Error())
		return false
	}
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

// sendResponse marshals a Packet and writes it with framing
func (p *ProtocolHandler) sendResponse(conn net.Conn, pkt *Packet) error {
	//TODO add signature
	pkt.Timestamp = time.Now().UTC().Format(time.RFC3339)
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

// validateTimestamp checks packet timestamp against current time (±60s) and sends error on failure
func (p *ProtocolHandler) validateTimestamp(ts string, conn net.Conn) bool {
	now := time.Now().UTC()
	pktTime, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		p.sendError(conn, ErrorCodeInvalidTimestamp, "invalid timestamp format")
		return false
	}
	// allow up to one minute skew in either direction
	if now.Sub(pktTime) > time.Minute || pktTime.Sub(now) > time.Minute {
		p.sendError(conn, ErrorCodeInvalidTimestamp, "timestamp out of acceptable range")
		return false
	}
	return true
}

// validateAntiSpam enforces per-packet anti-spam proof based on default policy
func (p *ProtocolHandler) validateAntiSpam(pkt *Packet, conn net.Conn) bool {
	policy, ok := DefaultRequiredAntiSpam[pkt.Type]
	// if no policy or policy set to none, skip validation
	if !ok || policy.Type == "none" {
		return true
	}
	// proof is required
	if pkt.AntiSpam == nil {
		p.sendError(conn, ErrorCodeSpamProofRequired, "missing anti_spam proof")
		return false
	}
	// proof parameters check
	proof := pkt.AntiSpam
	if proof.Type != policy.Type || proof.Bits != policy.Bits {
		p.sendError(conn, ErrorCodeInvalidSpamProof, "invalid anti_spam proof parameters")
		return false
	}

	hash := sha256.Sum256(pkt.Payload)
	resource := hex.EncodeToString(hash[:])

	if proof.Resource != resource {
		p.sendError(conn, ErrorCodeInvalidSpamProof, "anti_spam resource mismatch")
		return false
	}

	hc, err := hashcash.New(&hashcash.Resource{
		Data:          resource,
		ValidatorFunc: func(res string) bool { return true },
	}, &hashcash.Config{
		Bits: policy.Bits,
	})
	if err != nil {
		p.sendError(conn, ErrorCodeInternalServerError, "hashcash init failed")
		return false
	}
	valid, err := hc.Verify(proof.Nonce)
	if err != nil || !valid {
		p.sendError(conn, ErrorCodeInvalidSpamProof, "hashcash validation failed")
		return false
	}

	return true
}
