// pkg/transport/quill/protocol_handler.go
package quill

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
)

// handlerState represents the connection phase
// iota-based enum for efficiency

type handlerState int

const (
	stateAwaitHandshake handlerState = iota
	stateAwaitAuthOrKeys
	stateAuthenticated
)

// ProtocolHandler drives framing, state transitions, and delegates to ServiceHandler

type ProtocolHandler struct {
	svc *ServiceHandler
}

// NewProtocolHandler creates a new instance with the given service-layer handler
func NewProtocolHandler(svc *ServiceHandler) *ProtocolHandler {
	return &ProtocolHandler{svc: svc}
}

// Handle is the main connection loop: frame I/O, state machine, and delegation
func (p *ProtocolHandler) Handle(conn net.Conn) {
	defer conn.Close()

	var state handlerState = stateAwaitHandshake
	var ctx context.Context

	for {
		// 1. Read next packet with length-prefix framing
		pkt, err := readPacket(conn)
		if err != nil {
			return // I/O error or client closed
		}

		switch state {
		case stateAwaitHandshake:
			ack := p.doHandshake(pkt)
			writePacket(conn, ack)
			state = stateAwaitAuthOrKeys

		case stateAwaitAuthOrKeys:
			if pkt.Type == PacketTypeFetchKeys {
				resp := p.doFetchKeys(pkt)
				writePacket(conn, resp)

			} else if pkt.Type == PacketTypeAuth {
				ack, success := p.doAuth(pkt)
				writePacket(conn, ack)
				if !success {
					return // auth failed
				}
				// assume doAuth embeds new context in returnAck or side-channel
				ctx = context.Background() // TODO: extract real authenticated context
				state = stateAuthenticated

			} else {
				errPkt := buildErrorPacket(ErrorCodeMalformedPacket, "expected FETCH_KEYS or AUTH")
				writePacket(conn, errPkt)
				return
			}

		case stateAuthenticated:
			var resp *Packet
			// delegate to service layer
			switch pkt.Type {
			case PacketTypeSendEmail:
				resp = p.svc.HandleSendEmail(ctx, pkt)

			case PacketTypeFetchEmail:
				resp = p.svc.HandleFetchEmail(ctx, pkt)

			case PacketTypeUpdateEmail:
				resp = p.svc.HandleUpdateEmail(ctx, pkt)

			default:
				errPkt := buildErrorPacket(ErrorCodeMalformedPacket, "invalid op after auth")
				writePacket(conn, errPkt)
				return
			}

			writePacket(conn, resp)
		}
	}
}

// readPacket reads a single length-prefixed JSON packet
func readPacket(r io.Reader) (*Packet, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	var pkt Packet
	if err := json.Unmarshal(data, &pkt); err != nil {
		return nil, err
	}
	return &pkt, nil
}

// writePacket writes a Packet as length-prefixed JSON
func writePacket(w io.Writer, pkt *Packet) error {
	data, err := json.Marshal(pkt)
	if err != nil {
		return err
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// stubs for transport-level operations
func (p *ProtocolHandler) doHandshake(pkt *Packet) *Packet {
	// TODO: validate version, negotiate antispam, etc.
	return &Packet{Type: PacketTypeHandshakeAck, Payload: json.RawMessage(`{}`)}
}

func (p *ProtocolHandler) doFetchKeys(pkt *Packet) *Packet {
	// TODO: lookup keys for requested domains or users
	return &Packet{Type: PacketTypeKeyResponse, Payload: json.RawMessage(`{}`)}
}

func (p *ProtocolHandler) doAuth(pkt *Packet) (*Packet, bool) {
	// TODO: extract token, call authSvc.Authenticate, build ACK/NACK
	return &Packet{Type: PacketTypeAuthAck, Payload: json.RawMessage(`{}`)}, true
}

// buildErrorPacket constructs a generic ERROR packet with code and message
func buildErrorPacket(code, msg string) *Packet {
	errPayload := struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, msg}
	b, _ := json.Marshal(errPayload)
	return &Packet{Type: PacketTypeError, Payload: b}
}
