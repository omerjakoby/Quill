package quill

import (
	"log"
	"net"
)

// Handler defines the interface for handling a single connection's lifecycle
type Handler interface {
	Handle(conn net.Conn)
}

// Server manages the TCP server lifecycle
type Server struct {
	addr    string
	handler Handler
}

func NewServer(addr string, handler Handler) *Server {
	return &Server{
		addr:    addr,
		handler: handler,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
		return err
	}
	defer listener.Close()

	log.Printf("TCP server listening on %s", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("ERROR: could not accept connection: %v", err)
			continue
		}

		go s.handler.Handle(conn)
	}
}

//
