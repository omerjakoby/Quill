package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
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

// Packet mirrors the server's transport packet structure.
type Packet struct {
	Protocol     string          `json:"protocol"`
	Version      string          `json:"version"`
	Type         string          `json:"type"`
	SessionToken string          `json:"session_token,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

func main() {
	// Hardcoded JSON directory relative to this file's location
	jsonDir := os.Getenv("JSON_DIR")
	if jsonDir == "" {
		jsonDir = filepath.Join("..", "..", "tests", "Quill_Protocol_JSON", "Requests", "fetch")
	}

	// Prompt for Firebase token each run
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your Firebase authentication token: ")
	token, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read token: %v", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		log.Fatal("Token cannot be empty")
	}
	if !strings.HasPrefix(token, "Bearer ") {
		token = "Bearer " + token
	}

	// Collect JSON files from hardcoded directory
	pattern := filepath.Join(jsonDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("Failed to list JSON files in %s: %v", jsonDir, err)
	}
	if len(files) == 0 {
		log.Fatalf("No JSON files found in %s", jsonDir)
	}

	// Optional server address flag
	addr := flag.String("addr", "localhost:9876", "server address (host:port)")
	flag.Parse()

	// Send each packet file
	for _, file := range files {
		fmt.Printf("=== Sending %s ===\n", filepath.Base(file))

		raw, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Error reading %s: %v", file, err)
		}

		var pkt Packet
		if err := json.Unmarshal(raw, &pkt); err != nil {
			log.Fatalf("Invalid JSON in %s: %v", file, err)
		}

		// Override session token and timestamp
		pkt.SessionToken = token
		pkt.Timestamp = time.Now().UTC()

		prettyPrint("Request", pkt)

		// Send packet and receive response
		resp, err := sendAndReceiveTLS(*addr, &pkt)
		if err != nil {
			log.Fatalf("Error during send/receive: %v", err)
		}

		prettyPrint("Response", *resp)
		fmt.Println()
	}
}

// sendAndReceive connects to the server, sends the packet, and decodes the response.
func sendAndReceive(addr string, pkt *Packet) (*Packet, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(pkt); err != nil {
		return nil, fmt.Errorf("failed to send packet: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var resp Packet
	if err := decoder.Decode(&resp); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed by server")
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}

// sendAndReceiveTLS connects over TLS, sends pkt, and returns the decoded response.
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

// prettyPrint logs a packet with JSON indentation.
func prettyPrint(title string, pkt Packet) {
	fmt.Printf("%s Packet:\n", title)
	b, err := json.MarshalIndent(pkt, "", "  ")
	if err != nil {
		fmt.Printf("  <error marshalling packet>: %v\n", err)
		return
	}
	fmt.Println(string(b))
}
