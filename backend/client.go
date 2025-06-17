// File: client.go
package main

import (
	"encoding/json"
	"log"
	"net"
)

// Message must match the server's struct definition.
type Message struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
}

func main() {
	// IMPORTANT: Replace with your GCP instance's external IP address.
	const serverAddress = "34.19.74.247:8081"

	// Connect to the TCP server.
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a sample message to send.
	msg := Message{
		From:    "Omer",
		To:      "GCP Server",
		Content: "Hello from my local machine!",
	}

	// Marshal the message struct into JSON format.
	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	// CRITICAL STEP: Append the newline delimiter to the JSON data.
	// This is how our server knows the message is complete.
	jsonData = append(jsonData, '\n')

	// Send the JSON data to the server.
	_, err = conn.Write(jsonData)
	if err != nil {
		log.Fatalf("Failed to send data: %v", err)
	}

	log.Println("Message sent successfully!")
}
