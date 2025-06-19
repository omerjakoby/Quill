package quill

import (
	"context"
	"fmt"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type AuthService interface {
	// Validate the incoming token, and return a context carrying the UID
	Authenticate(ctx context.Context, token string) (context.Context, error)
}

type firebaseAuthService struct {
	firebaseAuthClient *auth.Client
}

type userContextKey struct{}

var userKey = userContextKey{}

func NewFirebaseAuthService(client *auth.Client) AuthService {
	return &firebaseAuthService{firebaseAuthClient: client}
}

func (s *firebaseAuthService) Authenticate(
	ctx context.Context,
	idToken string,
) (context.Context, error) {
	idToken = strings.TrimPrefix(idToken, "Bearer ")
	token, err := s.firebaseAuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify Firebase ID token: %w", err)
	}

	// Token is valid. Attach the Firebase User ID (UID) to the context.
	return context.WithValue(ctx, userKey, token.UID), nil
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(userKey)
	id, ok := v.(string)
	return id, ok
}

func InitAuthService(ctx context.Context, credPath string) (AuthService, error) {
	opt := option.WithCredentialsFile(credPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Auth: %w", err)
	}
	return &firebaseAuthService{firebaseAuthClient: client}, nil
}

func InitAuthServiceFromEnv(ctx context.Context, envPath string) (AuthService, error) {
	if err := godotenv.Load(envPath); err != nil {
		return nil, fmt.Errorf("loading .env: %w", err)
	}

	credPath := os.Getenv("firebase_service_account_path")
	if credPath == "" {
		return nil, fmt.Errorf("firebase_service_account_path not set")
	}

	// now call your existing InitAuthService
	return InitAuthService(ctx, credPath)
}
