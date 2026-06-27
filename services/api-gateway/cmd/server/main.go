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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/security"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	gatewayHandler, err := handler.NewGateway(cfg.Routes, cfg.UpstreamTimeout, logger)
	if err != nil {
		logger.Error("failed to create gateway", "error", err)
		os.Exit(1)
	}

	var jwtVerifier *security.JWTVerifier
	if cfg.JWTVerifyEnabled {
		jwtVerifier, err = security.NewJWTVerifier(
			cfg.JWKSURL,
			cfg.JWTIssuer,
			cfg.JWTAudience,
			cfg.JWKSCacheTTL,
			cfg.UpstreamTimeout,
		)
		if err != nil {
			logger.Error("failed to create jwt verifier", "error", err)
			os.Exit(1)
		}
		logger.Info("jwt verification enabled", "jwks_url", cfg.JWKSURL, "issuer", cfg.JWTIssuer, "audience", cfg.JWTAudience)
	} else {
		logger.Warn("jwt verification disabled; only use this for local development")
	}

	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", textHandler("ok\n"))
	mux.HandleFunc("/readyz", readinessHandler(gatewayHandler, jwtVerifier))
	mux.HandleFunc("/metrics", metrics.Handler())

	var apiHandler http.Handler = gatewayHandler
	if jwtVerifier != nil {
		apiHandler = security.AuthMiddleware(jwtVerifier, cfg.AuthRequiredPrefixes, apiHandler)
	}
	mux.Handle("/api/", apiHandler)

	var app http.Handler = mux
	app = observability.BodyLimitMiddleware(cfg.RequestBodyLimitBytes, app)
	app = observability.CORSMiddleware(cfg.CORSAllowedOrigins, app)
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(logger, app)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting api-gateway", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api-gateway stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api-gateway stopped")
}

func textHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
}

func readinessHandler(gateway *handler.Gateway, verifier *security.JWTVerifier) http.HandlerFunc {
	checks := []handler.ReadinessCheck{gateway.Ready}
	if verifier != nil {
		checks = append(checks, verifier.Ready)
	}
	return handler.NewReadinessHandler(checks...)
}
