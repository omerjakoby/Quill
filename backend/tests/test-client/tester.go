package main

import (
	"bufio"
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

	"github.com/joho/godotenv"
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
	jsonDir := "../Quill_Protocol_JSON/Requests"

	// Optional: load .env file
	_ = godotenv.Load("../../../.env")

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
		resp, err := sendAndReceive(*addr, &pkt)
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
