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
	SendEmail(ctx context.Context, req domain.SendEmailRequest) (domain.SendEmailResult, error)
	FetchEmail(ctx context.Context, req domain.FetchEmailRequest) (domain.FetchEmailResult, error)
	UpdateEmail(ctx context.Context, req domain.UpdateEmailRequest) (domain.UpdateEmailResult, error)
}

type KeyService interface {
	FetchKeys(ctx context.Context, req domain.FetchKeysRequest) (domain.FetchKeysResult, error)
}
