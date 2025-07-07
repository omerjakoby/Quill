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

var MAIL = "omer~quillmail.com"

func main() {
	// Flags: server address and JSON directory
	addr := flag.String("addr", "localhost:9876", "server address (host:port)")
	jsonDir := flag.String("dir", "C:/Users/GraphicsTel-Hashomer/GolandProjects/Quill/backend/tests/Quill_Protocol_JSON", "path to JSON request files")
	certFile := flag.String("cert", "C:/Users/GraphicsTel-Hashomer/GolandProjects/Quill/certificate/quill.crt", "path to CA certificate PEM")
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

	// Send test emails to populate the database
	sendTestEmails(conn, *jsonDir)

	// Run test cases for fetch_email_overview
	runFetchOverviewTests(conn, *jsonDir)
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

func sendTestEmails(conn net.Conn, jsonDir string) {
	fmt.Println("--- Sending Test Emails ---")

	// Email 1: To omer, with keyword 'project'
	sendModifiedEmail(conn, jsonDir, map[string]interface{}{
		"to":      []string{MAIL},
		"subject": "Project Update",
		"body":    map[string]string{"text": "Here is the latest on the project."},
	})

	// Email 2: From omer, with keyword 'deadline'
	sendModifiedEmail(conn, jsonDir, map[string]interface{}{
		"from":    MAIL,
		"to":      []string{"itamar~quillmail.com"},
		"subject": "Upcoming Deadline",
		"body":    map[string]string{"text": "Just a reminder about the deadline."},
	})

	// Email 3: To omer, with an attachment
	sendModifiedEmail(conn, jsonDir, map[string]interface{}{
		"to":      []string{MAIL},
		"subject": "Design Mockups",
		"body":    map[string]string{"text": "See attached mockups."},
		"attachments": []map[string]string{
			{"filename": "mockup.png", "mimetype": "image/png", "link": "http://example.com/mockup.png"},
		},
	})

	// Email 4: To omer, with the exact phrase 'team meeting'
	sendModifiedEmail(conn, jsonDir, map[string]interface{}{
		"to":      []string{MAIL},
		"subject": "Meeting Reminder",
		"body":    map[string]string{"text": "Don't forget the team meeting tomorrow."},
	})

	fmt.Println("--- Finished Sending Test Emails ---")
}

func sendModifiedEmail(conn net.Conn, jsonDir string, modifications map[string]interface{}) {
	packet, err := readPacketFromFile(filepath.Join(jsonDir, "send_email.json"))
	if err != nil {
		log.Printf("ERROR: Failed to read send_email.json: %v", err)
		return
	}

	// Always update the timestamp to the current time
	packet.Timestamp = time.Now().UTC().Format(time.RFC3339)

	var payload map[string]interface{}
	if err := json.Unmarshal(packet.Payload, &payload); err != nil {
		log.Printf("ERROR: Failed to unmarshal payload: %v", err)
		return
	}

	for key, value := range modifications {
		payload[key] = value
	}

	modifiedPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal modified payload: %v", err)
		return
	}
	packet.Payload = modifiedPayload

	sendPacket(conn, packet)
	receiveAndPrint(conn) // Wait for the response to ensure it's sent before moving on
}

func runFetchOverviewTests(conn net.Conn, jsonDir string) {
	fmt.Println("--- Running Fetch Email Overview Tests ---")

	// Test case 1: No filters (should return all emails for the user)
	fmt.Println("--- Test Case 1: No Filters ---")
	sendAndTestFetchOverview(conn, jsonDir, nil)

	// Test case 2: Filter by sender
	fmt.Println("--- Test Case 2: Filter by Sender ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"search": map[string]interface{}{
			"from": []string{MAIL},
		},
	})

	// Test case 3: Filter by recipient
	fmt.Println("--- Test Case 3: Filter by Recipient ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"search": map[string]interface{}{
			"to": []string{MAIL},
		},
	})

	// Test case 4: Filter by keyword
	fmt.Println("--- Test Case 4: Filter by Keyword ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"search": map[string]interface{}{
			"keywords": []string{"project"},
		},
	})

	// Test case 5: Filter by unread
	fmt.Println("--- Test Case 5: Filter by Unread ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"flags": map[string]interface{}{
			"is_read": false,
		},
	})

	// Test case 6: Filter by exact phrase
	fmt.Println("--- Test Case 6: Filter by Exact Phrase ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"search": map[string]interface{}{
			"exact_phrase": "team meeting",
		},
	})

	// Test case 7: Filter by has attachments
	fmt.Println("--- Test Case 7: Filter by Has Attachments ---")
	sendAndTestFetchOverview(conn, jsonDir, map[string]interface{}{
		"flags": map[string]interface{}{
			"has_attachments": true,
		},
	})
}

// sendAndTestFetchOverview sends a fetch_email_overview request with the specified filters
// and performs basic validation on the response.
func sendAndTestFetchOverview(conn net.Conn, jsonDir string, filters map[string]interface{}) {
	// 1. Read the base fetch_email_overview.json file
	basePacket, err := readPacketFromFile(filepath.Join(jsonDir, "fetch_email_overview.json"))
	if err != nil {
		log.Printf("ERROR: Failed to read base fetch_email_overview.json: %v", err)
		return
	}

	// 2. Modify the payload with the provided filters
	var payload map[string]interface{}
	if err := json.Unmarshal(basePacket.Payload, &payload); err != nil {
		log.Printf("ERROR: Failed to unmarshal base payload: %v", err)
		return
	}

	// If filters are provided, set them. Otherwise, remove the key.
	if filters != nil {
		payload["filters"] = filters
	} else {
		delete(payload, "filters")
	}

	// Marshal the modified payload back to JSON
	modifiedPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: Failed to marshal modified payload: %v", err)
		return
	}
	basePacket.Payload = modifiedPayload
	basePacket.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// 3. Send the modified packet
	fmt.Println("--- Sending Modified fetch_email_overview ---")
	sendPacket(conn, basePacket)
	prettyPrint("Sent", *basePacket)

	// 4. Receive and validate the response
	fmt.Println("--- Receiving Response ---")
	responsePacket, err := readPacket(conn)
	if err != nil {
		log.Printf("ERROR: Failed to receive response: %v", err)
		return
	}
	prettyPrint("Received", *responsePacket)

	// 5. Basic Validation
	var responsePayload map[string]interface{}
	if err := json.Unmarshal(responsePacket.Payload, &responsePayload); err != nil {
		log.Printf("ERROR: Failed to unmarshal response payload: %v", err)
		return
	}

	if status, ok := responsePayload["status"].(string); ok && status == "ERROR" {
		log.Printf("VALIDATION FAILED: Received ERROR response: %s", responsePacket.Payload)
		return
	}

	totalThreads, ok := responsePayload["total_threads"].(float64)
	if !ok {
		log.Printf("VALIDATION FAILED: 'total_threads' not found or not a number in response.")
	}

	overviews, ok := responsePayload["threads"].([]interface{})
	if !ok {
		log.Printf("VALIDATION FAILED: 'threads' not found or not an array in response.")
	}

	fmt.Printf("Validation Info: TotalThreads=%.0f, Received Overviews=%d\n", totalThreads, len(overviews))
	fmt.Println("--- Test Case Finished ---")
}

// readPacketFromFile reads a JSON file and unmarshals it into a Packet struct.
func readPacketFromFile(path string) (*Packet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}
	var pkt Packet
	if err := json.Unmarshal(raw, &pkt); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return &pkt, nil
}
