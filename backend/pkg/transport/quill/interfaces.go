// pkg/transport/quill/interfaces.go
package quill

import (
	"context"
	"quill/pkg/domain"
)

// authService authenticates tokens and returns a context enriched with identity.
type AuthService interface {
    Authenticate(ctx context.Context, token string) (context.Context, error)
}

// EmailService handles core email-domain operations: send, fetch, update.
type EmailService interface {
    Send(ctx context.Context, req domain.DomainSendRequest) (domain.DomainSendResult, error)
    Fetch(ctx context.Context, req domain.DomainFetchRequest) (domain.DomainFetchResult, error)
    Update(ctx context.Context, req domain.DomainUpdateRequest) (domain.DomainUpdateResult, error)
}
