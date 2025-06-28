package auth

import (
	"context"
	"fmt"
	"os"
	"quill/pkg/transport/quill"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type firebaseAuthService struct {
	firebaseAuthClient *auth.Client
}

func NewFirebaseAuthService(client *auth.Client) quill.AuthService {
	return &firebaseAuthService{firebaseAuthClient: client}
}

func (s *firebaseAuthService) Authenticate(ctx context.Context, idToken string) error {
	// strip any leading "Bearer "
	idToken = strings.TrimPrefix(idToken, "Bearer ")

	// verify with Firebase
	token, err := s.firebaseAuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		return fmt.Errorf("failed to verify Firebase ID token: %w", err)
	}

	// grab the mutable AuthInfo you seeded in protocol_handler.Serve()
	ai, ok := ctx.Value(quill.AuthInfoKey{}).(*quill.AuthInfo)
	if !ok {
		return fmt.Errorf("authentication context not initialized")
	}

	// store the UID for downstream handlers
	ai.UserID = token.UID
	return nil
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value("userID")
	id, ok := v.(string)
	return id, ok
}

func InitAuthService(ctx context.Context, credPath string) (quill.AuthService, error) {
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

func InitAuthServiceFromEnv(ctx context.Context, envPath string) (quill.AuthService, error) {
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
