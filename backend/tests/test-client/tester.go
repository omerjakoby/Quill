package main

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/umahmood/hashcash"
)

// Packet matches the Quill DTO definition in protocol
// Includes optional AntiSpam proof negotiated at handshake
type Packet struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature *string         `json:"signature,omitempty"`
	AntiSpam  *AntiSpamProof  `json:"anti_spam,omitempty"`
}

// AntiSpamProof carries proof-of-work info for hashcash
type AntiSpamProof struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Bits     int    `json:"bits"`
	Nonce    string `json:"nonce"`
}

// handshakeAckPayload is used to parse the server's HANDSHAKE_ACK
// and extract the per-command anti-spam policy
type handshakeAckPayload struct {
	Accepted         bool   `json:"accepted"`
	Version          string `json:"version"`
	RequiredAntiSpam map[string]struct {
		Type string `json:"type"`
		Bits int    `json:"bits"`
	} `json:"required_anti_spam"`
	Options struct {
		Encryption bool `json:"encryption"`
	} `json:"options"`
}

// requiredAntiSpam holds the policy received in HANDSHAKE_ACK
// mapping Packet.Type to the required method and bits
var requiredAntiSpam = make(map[string]struct {
	Type string
	Bits int
})

func main() {
	// Command-line flags
	addr := flag.String("addr", "localhost:9876", "server address (host:port)")
	jsonDir := flag.String("dir", "tests/Quill_Protocol_JSON", "path to JSON request files")
	certFile := flag.String("cert", "../certificate/quill.crt", "path to CA certificate PEM")
	flag.Parse()

	// Prompt for auth token
	token := promptToken()

	// Establish TLS connection
	conn := dialTLS(*addr, *certFile)
	defer conn.Close()

	// 1) HANDSHAKE
	sendJSONFile(conn, filepath.Join(*jsonDir, "handshake.json"), nil)
	pkt := receivePacket(conn)
	prettyPrint("Received", *pkt)
	if pkt.Type == "HANDSHAKE_ACK" {
		var ack handshakeAckPayload
		if err := json.Unmarshal(pkt.Payload, &ack); err != nil {
			log.Fatalf("Invalid HANDSHAKE_ACK payload: %v", err)
		}
		for cmd, p := range ack.RequiredAntiSpam {
			requiredAntiSpam[cmd] = struct {
				Type string
				Bits int
			}{Type: p.Type, Bits: p.Bits}
		}
	}

	// 2) AUTH
	sendJSONFile(conn, filepath.Join(*jsonDir, "auth.json"), func(pkt *Packet) {
		var auth map[string]interface{}
		json.Unmarshal(pkt.Payload, &auth)
		creds := auth["credentials"].(map[string]interface{})
		creds["token"] = token
		newPayload, _ := json.Marshal(auth)
		pkt.Payload = newPayload
	})
	receiveAndPrint(conn)

	// 3) Remaining commands
	entries, err := os.ReadDir(*jsonDir)
	if err != nil {
		log.Fatalf("Failed to read dir %s: %v", *jsonDir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "handshake.json" || name == "auth.json" || filepath.Ext(name) != ".json" {
			continue
		}
		fmt.Printf("=== %s ===\n", name)
		sendJSONFile(conn, filepath.Join(*jsonDir, name), nil)
		receiveAndPrint(conn)
	}
}

// promptToken reads and formats the bearer token
func promptToken() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your session token: ")
	tkn, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read token: %v", err)
	}
	tkn = strings.TrimSpace(tkn)
	if !strings.HasPrefix(tkn, "Bearer ") {
		tkn = "Bearer " + tkn
	}
	return tkn
}

// dialTLS sets up a TLS connection trusting the given CA cert
func dialTLS(addr, caPath string) net.Conn {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		log.Fatalf("Could not read CA file %s: %v", caPath, err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		log.Fatalf("Failed to parse CA cert")
	}
	tlsCfg := &tls.Config{RootCAs: roots, InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		log.Fatalf("TLS dial failed: %v", err)
	}
	return conn
}

// sendJSONFile reads a JSON file into Packet, applies optional modifier,
// computes hashcash if required, and sends it
func sendJSONFile(conn net.Conn, path string, modify func(*Packet)) {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading %s: %v", path, err)
	}
	var pkt Packet
	if err := json.Unmarshal(raw, &pkt); err != nil {
		log.Fatalf("Invalid JSON in %s: %v", path, err)
	}
	// Override timestamp
	pkt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	// Apply any payload modifications
	if modify != nil {
		modify(&pkt)
	}
	// Compute and attach hashcash proof if required
	if policy, ok := requiredAntiSpam[pkt.Type]; ok && policy.Type == "hashcash" {
		addHashcash(&pkt, policy.Bits)
	}
	sendPacket(conn, &pkt)
	prettyPrint("Sent", pkt)
}

// sendPacket writes a Packet with length-prefix framing
func sendPacket(conn net.Conn, pkt *Packet) {
	raw, _ := json.Marshal(pkt)
	buf := make([]byte, 4+len(raw))
	binary.BigEndian.PutUint32(buf, uint32(len(raw)))
	copy(buf[4:], raw)
	if _, err := conn.Write(buf); err != nil {
		log.Fatalf("Failed to write packet: %v", err)
	}
}

// receivePacket reads one Packet from conn and returns it
func receivePacket(conn net.Conn) *Packet {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		log.Fatalf("Receive error: %v", err)
	}
	length := binary.BigEndian.Uint32(lenBuf)
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		log.Fatalf("Receive error: %v", err)
	}
	var pkt Packet
	if err := json.Unmarshal(body, &pkt); err != nil {
		log.Fatalf("Invalid packet JSON: %v", err)
	}
	return &pkt
}

// receiveAndPrint reads one Packet and pretty-prints it
func receiveAndPrint(conn net.Conn) {
	pkt := receivePacket(conn)
	prettyPrint("Received", *pkt)
}

// prettyPrint outputs Packet in indented JSON
func prettyPrint(prefix string, pkt Packet) {
	fmt.Printf("%s Packet: %s\n", prefix, pkt.Type)
	b, _ := json.MarshalIndent(pkt, "", "  ")
	fmt.Println(string(b))
}

func addHashcash(pkt *Packet, bits int) {
	// 1) Unmarshal the raw payload to normalize key ordering & remove whitespace
	var payload interface{}
	if err := json.Unmarshal(pkt.Payload, &payload); err != nil {
		log.Fatalf("hashcash: failed to unmarshal payload: %v", err)
	}

	// 2) Re-marshal into compact, sorted JSON to get canonical form
	canonical, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("hashcash: failed to marshal canonical payload: %v", err)
	}
	fmt.Printf("client Canonical: %s\n", string(canonical))

	// 3) Compute resource = sha256(canonical JSON)
	sum := sha256.Sum256(canonical)
	resource := hex.EncodeToString(sum[:])

	// 4) Create & solve the proof using the correct library
	hc, err := hashcash.New(&hashcash.Resource{
		Data:          resource,
		ValidatorFunc: func(_ string) bool { return true },
	}, &hashcash.Config{
		Bits: 18,
	})
	if err != nil {
		log.Fatalf("hashcash init failed: %v", err)
	}

	// 5) Compute the nonce
	solution, err := hc.Compute()
	if err != nil {
		log.Fatalf("hashcash compute failed: %v", err)
	}
	// 6) Attach proof to the packet
	pkt.AntiSpam = &AntiSpamProof{
		Type:     "hashcash",
		Resource: resource,
		Bits:     bits,
		Nonce:    solution,
	}
}
