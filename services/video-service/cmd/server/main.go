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
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/event"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/service"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/storage"
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

	store, closeStore, err := openStore(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("failed to initialize video store", "service", serviceName, "error", err)
		os.Exit(1)
	}
	defer closeStore()

	metrics := observability.NewMetrics()
	uploadSigner, err := newUploadSigner(cfg)
	if err != nil {
		logger.Error("failed to initialize upload signer", "service", serviceName, "error", err)
		os.Exit(1)
	}
	videoService := service.NewVideoService(store, service.Options{
		Environment:      cfg.Environment,
		RawVideoBucket:   cfg.RawVideoBucket,
		UploadURLBase:    cfg.UploadURLBase,
		UploadRequestTTL: cfg.UploadRequestTTL,
		PresignedTTL:     cfg.PresignedUploadTTL,
		UploadSigner:     uploadSigner,
		Metrics:          metrics,
		Logger:           logger,
	})

	mux := http.NewServeMux()
	handler.New(videoService, handler.Options{InternalAPIToken: cfg.InternalAPIToken}).RegisterRoutes(mux, metrics.Handler())

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

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	closePublisher := startOutboxPublisher(rootCtx, cfg, store, metrics, logger)
	defer closePublisher()

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
	cancelRoot()

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
		logger.Info("using postgres video store", "service", serviceName)
		return store, func() {
			if err := store.Close(); err != nil {
				logger.Error("failed to close video store", "service", serviceName, "error", err)
			}
		}, nil
	}

	logger.Warn("using in-memory video store; this is only suitable for local development", "service", serviceName, "environment", cfg.Environment)
	return repository.NewMemoryStore(), func() {}, nil
}

func newUploadSigner(cfg config.Config) (storage.UploadSigner, error) {
	if !cfg.UseMinIOPresigner() {
		return nil, nil
	}
	return storage.NewS3Presigner(storage.S3PresignerConfig{
		Endpoint:  cfg.MinIOEndpoint,
		AccessKey: cfg.MinIOAccessKey,
		SecretKey: cfg.MinIOSecretKey,
		Region:    cfg.MinIORegion,
		UseSSL:    cfg.MinIOUseSSL,
	})
}

func startOutboxPublisher(ctx context.Context, cfg config.Config, store repository.Store, metrics *observability.Metrics, logger *slog.Logger) func() {
	if !cfg.OutboxPublisher {
		logger.Info("outbox publisher disabled", "service", serviceName)
		return func() {}
	}
	publisher, err := event.NewKafkaPublisher(event.KafkaPublisherConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.VideoEventsTopic,
	})
	if err != nil {
		logger.Error("failed to initialize outbox publisher", "service", serviceName, "error", err)
		os.Exit(1)
	}
	worker := event.NewOutboxWorker(event.OutboxWorkerConfig{
		Store:        store,
		Publisher:    publisher,
		Logger:       logger,
		Metrics:      metrics,
		PollInterval: cfg.OutboxPollInterval,
		BatchSize:    cfg.OutboxBatchSize,
	})
	go worker.Run(ctx)
	return func() {
		if err := publisher.Close(); err != nil {
			logger.Error("failed to close outbox publisher", "service", serviceName, "error", err)
		}
	}
}
