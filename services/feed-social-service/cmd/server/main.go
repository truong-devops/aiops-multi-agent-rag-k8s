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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/service"
)

const serviceName = "feed-social-service"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	metrics := observability.NewMetrics()
	store, closeStore, err := openStore(context.Background(), cfg, logger, metrics)
	if err != nil {
		logger.Error("failed to initialize feed store", "service", serviceName, "error", err)
		os.Exit(1)
	}
	defer closeStore()

	feedService := service.NewFeedService(store, service.Options{
		Metrics: metrics,
		Logger:  logger,
	})

	mux := http.NewServeMux()
	handler.New(feedService).RegisterRoutes(mux, metrics.Handler())

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

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger, metrics *observability.Metrics) (repository.Store, func(), error) {
	if cfg.UsePostgres() {
		store, err := repository.NewPostgresStore(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		logger.Info("using postgres feed store", "service", serviceName)
		instrumented := repository.NewInstrumentedStore(store, metrics)
		return instrumented, func() {
			if err := store.Close(); err != nil {
				logger.Error("failed to close feed store", "service", serviceName, "error", err)
			}
		}, nil
	}

	logger.Warn("using in-memory feed store; this is only suitable for local development", "service", serviceName, "environment", cfg.Environment)
	return repository.NewInstrumentedStore(repository.NewMemoryStore(), metrics), func() {}, nil
}
