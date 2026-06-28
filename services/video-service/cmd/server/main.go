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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/service"
)

const serviceName = "video-service"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	store := repository.NewMemoryStore()
	videoService := service.NewVideoService(store, service.Options{
		RawVideoBucket:   cfg.RawVideoBucket,
		UploadURLBase:    cfg.UploadURLBase,
		UploadRequestTTL: cfg.UploadRequestTTL,
	})

	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	handler.New(videoService).RegisterRoutes(mux, metrics.Handler())

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
