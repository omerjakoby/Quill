package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv" // For converting strings to integers
)

// sumHandler handles requests to the /sum endpoint.
// It expects two query parameters: 'a' and 'b'.
func sumHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	aStr := query.Get("a")
	bStr := query.Get("b")

	// Validate and convert 'a'
	a, err := strconv.Atoi(aStr)
	if err != nil {
		http.Error(w, "Invalid number for 'a'", http.StatusBadRequest)
		log.Printf("Error converting 'a': %v", err)
		return
	}

	// Validate and convert 'b'
	b, err := strconv.Atoi(bStr)
	if err != nil {
		http.Error(w, "Invalid number for 'b'", http.StatusBadRequest)
		log.Printf("Error converting 'b': %v", err)
		return
	}

	// Calculate the sum
	sum := a + b

	// Send the response
	fmt.Fprintf(w, "The sum of %d and %d is %d\n", a, b, sum)
	log.Printf("Calculated sum: %d + %d = %d", a, b, sum)
}

func main() {
	// Register the handler for the /sum endpoint
	http.HandleFunc("/sum", sumHandler)

	// Register a simple root handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Welcome to the Go Sum Server! Try /sum?a=5&b=10")
	})

	// Define the port to listen on
	const port = "8080"
	log.Printf("Go server starting on port %s...", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
