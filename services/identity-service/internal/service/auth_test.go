package service

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/security"
)

func TestRefreshRotationPreservesSessionExpiry(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	jwtManager, err := security.NewJWTManager("aiops-video-platform", "aiops-api", "")
	if err != nil {
		t.Fatalf("NewJWTManager() error = %v", err)
	}

	auth := NewAuthService(store, jwtManager, time.Minute, time.Hour)
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }

	if _, err := auth.Register(ctx, RegisterInput{
		Email:    "refresh-expiry@example.com",
		Password: "strong-password",
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	loginResult, err := auth.Login(ctx, LoginInput{
		Email:    "refresh-expiry@example.com",
		Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	sessionExpiresAt := now.Add(time.Hour)
	now = now.Add(30 * time.Minute)
	refreshResult, err := auth.Refresh(ctx, loginResult.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	token, session, _, err := store.FindRefreshTokenByHash(ctx, security.HashRefreshToken(refreshResult.RefreshToken))
	if err != nil {
		t.Fatalf("FindRefreshTokenByHash() error = %v", err)
	}
	if !session.ExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("session expires at = %s, want %s", session.ExpiresAt, sessionExpiresAt)
	}
	if !token.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("rotated token expires at = %s, want session expiry %s", token.ExpiresAt, session.ExpiresAt)
	}
	if token.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("rotated token extended session expiry to %s", token.ExpiresAt)
	}
}
