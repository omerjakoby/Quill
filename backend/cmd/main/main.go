package main

import (
	"log"
	"quill/pkg/transport/quill"
)

func main() {
    // 1. Instantiate your services.
    // These are the concrete implementations of the interfaces required by the MessageHandler.
    authSvc := &authService{}
    // msgSvc := &mockMessageService{}

    // 2. Instantiate the handler and inject its dependencies.
    // The handler is responsible for the protocol-level logic of your server.
    // messageHandler := quill.NewMessageHandler(authSvc, msgSvc)

    // 3. Configure and create the server instance.
    // The server is the generic TCP component that listens for connections.
    serverAddr := "localhost:9876"
    server := quill.NewServer(serverAddr, messageHandler)

    // 4. Start the server.
    // This will begin the blocking loop to accept and handle new connections.
    log.Printf("INFO: starting Quill protocol server on %s", serverAddr)
    if err := server.Start(); err != nil {
        log.Fatalf("FATAL: could not start server: %v", err)
    }
}
