package quill

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

const port = ":8081"

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("Serving %s\n", conn.RemoteAddr().String())
	reader := bufio.NewReader(conn)
	jsonData, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("Client disconnected or error: %v", err)
		return
	}
	var msg Packet
	err = json.Unmarshal(jsonData, &msg)
	if err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		return
	}
	fmt.Println("--- New Message Received ---")
	fmt.Printf("protocol: %s\n", msg.Protocol)
	fmt.Printf("Type: %s\n", msg.Type)
	fmt.Printf("payload: %s\n", msg.Payload)
	fmt.Println("--------------------------")
}

func OpenTcpPort() {
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
