package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/security"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/service"
)

const serviceName = "identity-service"

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	jwtManager, err := security.NewJWTManager(cfg.Issuer, cfg.Audience, cfg.SigningKeyPEM)
	if err != nil {
		logger.Error("failed to initialize jwt signer", "error", err)
		os.Exit(1)
	}

	store := repository.NewMemoryStore()
	authService := service.NewAuthService(store, jwtManager, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	googleService := service.NewGoogleOAuthService(store, service.GoogleOAuthConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		AuthURL:      cfg.GoogleAuthURL,
		TokenURL:     cfg.GoogleTokenURL,
		JWKSURL:      cfg.GoogleJWKSURL,
		Scopes:       cfg.GoogleScopes,
	})

	mux := http.NewServeMux()
	handler.New(authService, googleService).RegisterRoutes(mux)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           observability.RequestContextMiddleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting service", "service", serviceName, "port", cfg.Port, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("service stopped unexpectedly", "service", serviceName, "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "service", serviceName, "error", err)
		os.Exit(1)
	}
	logger.Info("service stopped", "service", serviceName)
}
