package quill

import (
	"crypto/tls"
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

func (s *Server) StartTLS(certFile, keyFile string) error {
	// Load your X.509 certificate and private key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("failed to load cert/key pair: %v", err)
		return err
	}

	// Configure TLS settings (e.g. require at least TLS1.2)
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Create a TLS listener instead of a plain TCP one
	listener, err := tls.Listen("tcp", s.addr, tlsCfg)
	if err != nil {
		log.Fatalf("failed to start TLS listener: %v", err)
		return err
	}
	defer listener.Close()

	log.Printf("TLS server listening on %s", s.addr)

	// Accept and handle connections just like in Start()
	for {
		conn, err := listener.Accept() // returns a *tls.Conn
		if err != nil {
			log.Printf("ERROR: could not accept TLS connection: %v", err)
			continue
		}
		go s.handler.Handle(conn)
	}
}
