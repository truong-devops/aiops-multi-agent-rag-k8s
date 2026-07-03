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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/service"
)

const serviceName = "media-worker"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	store, closeStore, err := openStore(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("failed to initialize processing store", "service", serviceName, "error", err)
		os.Exit(1)
	}
	defer closeStore()

	processingService := service.NewProcessingService(store, service.Options{
		RawBucket:   cfg.RawVideoBucket,
		MaxAttempts: cfg.MaxAttempts,
		Logger:      logger,
	})
	metrics := observability.NewMetrics()

	mux := http.NewServeMux()
	handler.New(processingService).RegisterRoutes(mux, metrics.Handler())

	var app http.Handler = mux
	app = observability.BodyLimitMiddleware(cfg.RequestBodyLimitBytes, app)
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(logger, app)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
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

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (repository.Store, func(), error) {
	if cfg.UsePostgres() {
		store, err := repository.NewPostgresStore(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		logger.Info("using postgres processing store", "service", serviceName)
		return store, func() {
			if err := store.Close(); err != nil {
				logger.Error("failed to close processing store", "service", serviceName, "error", err)
			}
		}, nil
	}

	logger.Warn("using in-memory processing store; this is only suitable for local development", "service", serviceName, "environment", cfg.Environment)
	return repository.NewMemoryStore(), func() {}, nil
}
