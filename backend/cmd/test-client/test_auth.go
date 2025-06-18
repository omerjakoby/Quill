package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Define the same packet structure as in your server
type Packet struct {
	Protocol     string          `json:"protocol"`
	Version      string          `json:"version"`
	Type         string          `json:"type"`
	SessionToken string          `json:"session_token,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
	Payload      json.RawMessage `json:"payload"`
}

// Define a simple payload for testing
type FetchPayload struct {
	Mode     string `json:"mode"`
	ThreadID string `json:"thread_id,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func main() {
	runTest("")
}

func runTest(savedToken string) {
	// Load environment variables
	err := godotenv.Load("C:\\Users\\assij\\GolandProjects\\Quill\\.env")
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	var token string

	// Only prompt for token if we don't have a saved one
	if savedToken == "" {
		// Prompt user for Firebase token
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your Firebase authentication token: ")
		token, err = reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read token: %v", err)
		}
		token = strings.TrimSpace(token)

		if token == "" {
			log.Fatal("Token cannot be empty")
		}

		fmt.Println("Token received successfully")
	} else {
		token = savedToken
		fmt.Println("Using previously entered token")
	}

	// Connect to the Quill server
	conn, err := net.Dial("tcp", "localhost:9876")
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	fmt.Println("Connected to Quill server at localhost:9876")

	// Create a FETCH request packet with authentication
	fetchPayload := FetchPayload{
		Mode:  "thread",
		Limit: 10,
	}
	payloadBytes, err := json.Marshal(fetchPayload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	packet := Packet{
		Protocol:     "quill",
		Version:      "1.0",
		Type:         "FETCH",
		SessionToken: token,
		Timestamp:    time.Now().UTC(),
		Payload:      payloadBytes,
	}

	// Send the packet to the server
	encoder := json.NewEncoder(conn)
	err = encoder.Encode(packet)
	if err != nil {
		log.Fatalf("Failed to send packet: %v", err)
	}
	fmt.Println("Sent FETCH request with authentication token")

	// Read and display the response
	decoder := json.NewDecoder(conn)
	var responsePacket Packet
	err = decoder.Decode(&responsePacket)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Pretty print the response
	fmt.Println("\nServer Response:")
	fmt.Printf("Type: %s\n", responsePacket.Type)
	fmt.Printf("Timestamp: %s\n", responsePacket.Timestamp)

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, responsePacket.Payload, "", "  ")
	if err != nil {
		fmt.Printf("Raw Payload: %s\n", string(responsePacket.Payload))
	} else {
		fmt.Printf("Payload: %s\n", prettyJSON.String())
	}

	fmt.Println("\nTest completed successfully!")

	// Ask if user wants to run again
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nDo you want to run again? (y/n): ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "y" || answer == "yes" {
		runTest(token) // Run again with the same token
	} else {
		fmt.Println("Exiting program.")
	}
}

// for commit
