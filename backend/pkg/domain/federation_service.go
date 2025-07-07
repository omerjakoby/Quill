package domain

import (
	"context"
	"log"
	"strings"
)

// FederationSender handles sending emails to external Quill servers
type FederationSender interface {
	SendFederatedEmail(ctx context.Context, req FederatedEmailRequest) error
}

// FederatedEmailRequest represents an email to be sent to an external server
type FederatedEmailRequest struct {
	TargetDomain string       // the domain to send to (e.g., "another-server.com")
	Recipients   []string     // list of recipients on this domain
	From         string       // sender address
	Subject      string       // email subject
	Body         EmailBody    // email content
	MessageID    string       // unique message ID
	ThreadID     *string      // thread ID if part of existing thread
	Attachments  []Attachment // attachments
}

// MockFederationSender is a mock implementation that logs instead of making real network calls
// TODO
type MockFederationSender struct{}

// NewMockFederationSender creates a new mock federation sender
func NewMockFederationSender() *MockFederationSender {
	return &MockFederationSender{}
}

// SendFederatedEmail implements the FederationSender interface with logging
func (m *MockFederationSender) SendFederatedEmail(_ context.Context, req FederatedEmailRequest) error {
	log.Printf("FEDERATION: Sending email to external domain: %s", req.TargetDomain)
	log.Printf("FEDERATION: Recipients: %v", req.Recipients)
	log.Printf("FEDERATION: From: %s", req.From)
	log.Printf("FEDERATION: Subject: %s", req.Subject)
	log.Printf("FEDERATION: MessageID: %s", req.MessageID)
	if req.ThreadID != nil {
		log.Printf("FEDERATION: ThreadID: %s", *req.ThreadID)
	}
	log.Printf("FEDERATION: Body length - Text: %d chars, HTML: %d chars",
		len(req.Body.Text), len(req.Body.HTML))
	if len(req.Attachments) > 0 {
		log.Printf("FEDERATION: Attachments: %d", len(req.Attachments))
		for i, att := range req.Attachments {
			log.Printf("FEDERATION: Attachment %d: %s (%s)", i+1, att.Filename, att.Mimetype)
		}
	}
	log.Printf("FEDERATION: Mock delivery to %s completed successfully", req.TargetDomain)
	return nil
}

// GroupRecipientsByDomain groups a list of email addresses by their domain
func GroupRecipientsByDomain(recipients []string) map[string][]string {
	groups := make(map[string][]string)

	for _, recipient := range recipients {
		domain := extractEmailDomain(recipient)
		if domain != "" {
			groups[domain] = append(groups[domain], recipient)
		}
	}

	return groups
}

// extractEmailDomain extracts the domain portion from an email address
func extractEmailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// SendToExternalDomains handles the federated delivery of emails to external domains
func SendToExternalDomains(ctx context.Context, federationSender FederationSender,
	queuedRecipients []string, req SendEmailRequest) {

	if len(queuedRecipients) == 0 {
		return
	}

	// Group recipients by domain to minimize network calls
	domainGroups := GroupRecipientsByDomain(queuedRecipients)

	for targetDomain, recipients := range domainGroups {
		federatedReq := FederatedEmailRequest{
			TargetDomain: targetDomain,
			Recipients:   recipients,
			From:         req.From,
			Subject:      req.Subject,
			Body:         req.Body,
			MessageID:    req.MessageID,
			ThreadID:     req.ThreadID,
			Attachments:  req.Attachments,
		}

		// Send the federated email (this is non-blocking since it's called in a goroutine)
		if err := federationSender.SendFederatedEmail(ctx, federatedReq); err != nil {
			log.Printf("FEDERATION ERROR: Failed to send email to domain %s: %v", targetDomain, err)
		}
	}
}
