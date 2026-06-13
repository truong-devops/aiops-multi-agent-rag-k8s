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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", textHandler("ok\n"))
	mux.HandleFunc("/readyz", textHandler("ready\n"))
	mux.HandleFunc("/metrics", textHandler("# metrics placeholder\n"))

	gatewayHandler, err := handler.NewGateway(cfg.Routes, logger)
	if err != nil {
		logger.Error("failed to create gateway", "error", err)
		os.Exit(1)
	}
	mux.Handle("/api/", gatewayHandler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           observability.RequestContextMiddleware(logger, mux),
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
