package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("Serving %s\n", conn.RemoteAddr().String())
	reader := bufio.NewReader(conn)
	jsonData, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("Client disconnected or error: %v", err)
		return
	}
	var msg Message
	err = json.Unmarshal(jsonData, &msg)
	if err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		return
	}
	fmt.Println("--- New Message Received ---")
	fmt.Printf("From: %s\n", msg.From)
	fmt.Printf("To: %s\n", msg.To)
	fmt.Printf("Content: %s\n", msg.Content)
	fmt.Println("--------------------------")
}

func main() {
	const port = ":8081"
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("TCP server listening on port %s", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}
