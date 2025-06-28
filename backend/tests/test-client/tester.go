package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
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
)

// Packet matches the Quill DTO definition in protocol
type Packet struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature *string         `json:"signature,omitempty"`
	AntiSpam  *AntiSpamProof  `json:"anti_spam,omitempty"`
}

// AntiSpamProof carries proof-of-work info
type AntiSpamProof struct {
	Type     string `json:"type"`
	Resource string `json:"resource"`
	Bits     int    `json:"bits"`
	Nonce    string `json:"nonce"`
}

func main() {
	// Flags: server address and JSON directory
	addr := flag.String("addr", "localhost:9876", "server address (host:port)")
	jsonDir := flag.String("dir", "./tests/requests", "path to JSON request files")
	certFile := flag.String("cert", "../certificate/quill.crt", "path to CA certificate PEM")
	flag.Parse()

	// Prompt for Firebase token for AUTH
	token := promptToken()

	// Establish persistent TLS connection
	conn := dialTLS(*addr, *certFile)
	defer conn.Close()

	// Send HANDSHAKE
	sendJSONFile(conn, filepath.Join(*jsonDir, "handshake.json"), nil)
	receiveAndPrint(conn)

	// Send AUTH (inject token)
	sendJSONFile(conn, filepath.Join(*jsonDir, "auth.json"), func(pkt *Packet) {
		// replace credentials.token in payload
		var auth map[string]interface{}
		json.Unmarshal(pkt.Payload, &auth)
		creds := auth["credentials"].(map[string]interface{})
		creds["token"] = token
		newPayload, _ := json.Marshal(auth)
		pkt.Payload = newPayload
	})
	receiveAndPrint(conn)

	// Iterate through remaining JSON files
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

// sendJSONFile reads a JSON file into Packet and sends it
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
	if modify != nil {
		modify(&pkt)
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

// receiveAndPrint reads one Packet from conn and prints it
func receiveAndPrint(conn net.Conn) {
	pkt, err := readPacket(conn)
	if err != nil {
		log.Fatalf("Receive error: %v", err)
	}
	prettyPrint("Received", *pkt)
}

// readPacket reads length-prefixed frame and unmarshals Packet
func readPacket(conn net.Conn) (*Packet, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	var pkt Packet
	if err := json.Unmarshal(body, &pkt); err != nil {
		return nil, err
	}
	return &pkt, nil
}

// prettyPrint outputs Packet in indented JSON
func prettyPrint(prefix string, pkt Packet) {
	fmt.Printf("%s Packet: %s\n", prefix, pkt.Type)
	b, _ := json.MarshalIndent(pkt, "", "  ")
	fmt.Println(string(b))
}
