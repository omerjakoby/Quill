package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type CreateUserRequest struct {
	UserQuillMail string `json:"userQuillMail"`
	UserEmail     string `json:"userEmail"`
	UsersUID      string `json:"usersUID"`
	AuthToken     string `json:"authToken"` // Firebase authentication token
}

type CreateUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"userID,omitempty"`
}

func main() {
	// Prompt for user details
	var quillMail, email, uid, authToken string
	quillMail = "omer~quillmail.xyz" // Replace with actual input or prompt
	email = "omer.jakoby@gmail.com"
	uid = "ftjphktn8mfiG4TWD1voZi4GCsU2"
	fmt.Print("Enter your Firebase authentication token: ")
	fmt.Scan(&authToken)

	reqBody := CreateUserRequest{
		UserQuillMail: quillMail,
		UserEmail:     email,
		UsersUID:      uid,
		AuthToken:     authToken, // Include the Firebase token in the request
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	// Allow self-signed certs for local testing
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	url := "https://localhost:8080/createUser"
	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	var respBody CreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		log.Printf("Failed to decode response: %v", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %+v\n", respBody)
}
