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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/client"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/event"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/handler"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/processor"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/service"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/storage"
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

	metrics := observability.NewMetrics()
	objectStore, err := newObjectStore(cfg)
	if err != nil {
		logger.Error("failed to initialize object store", "service", serviceName, "error", err)
		os.Exit(1)
	}
	statusClient, err := newStatusClient(cfg)
	if err != nil {
		logger.Error("failed to initialize video status client", "service", serviceName, "error", err)
		os.Exit(1)
	}
	videoProcessor, err := newProcessor(cfg, objectStore)
	if err != nil {
		logger.Error("failed to initialize processor", "service", serviceName, "error", err)
		os.Exit(1)
	}
	processingService := service.NewProcessingService(store, service.Options{
		RawBucket:    cfg.RawVideoBucket,
		MaxAttempts:  cfg.MaxAttempts,
		WorkerID:     cfg.WorkerID,
		LeaseTTL:     cfg.JobLeaseTTL,
		BatchSize:    cfg.JobBatchSize,
		Processor:    videoProcessor,
		StatusClient: statusClient,
		Metrics:      metrics,
		Logger:       logger,
	})

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

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	closeConsumer := startUploadedConsumer(rootCtx, cfg, processingService, metrics, logger)
	defer closeConsumer()
	if cfg.RunnerEnabled {
		go processingService.Run(rootCtx, cfg.JobPollInterval)
	} else {
		logger.Info("processing runner disabled", "service", serviceName)
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

func newObjectStore(cfg config.Config) (storage.ObjectStore, error) {
	if cfg.MinIOEndpoint == "" {
		return storage.NoopObjectStore{}, nil
	}
	return storage.NewS3ObjectStore(storage.S3ObjectStoreConfig{
		Endpoint:  cfg.MinIOEndpoint,
		AccessKey: cfg.MinIOAccessKey,
		SecretKey: cfg.MinIOSecretKey,
		Region:    cfg.MinIORegion,
		UseSSL:    cfg.MinIOUseSSL,
	})
}

func newProcessor(cfg config.Config, objectStore storage.ObjectStore) (processor.Processor, error) {
	if cfg.ProcessingMode == "ffmpeg" {
		return processor.NewFFmpegProcessor(processor.FFmpegConfig{
			ObjectStore:     objectStore,
			ProcessedBucket: cfg.ProcessedVideoBucket,
			ThumbnailBucket: cfg.ThumbnailBucket,
			FFmpegPath:      cfg.FFmpegPath,
			FFprobePath:     cfg.FFprobePath,
			Timeout:         cfg.ProcessingTimeout,
		})
	}
	return processor.NewPlaceholderProcessor(processor.PlaceholderConfig{
		ObjectStore: objectStore,
	}), nil
}

func newStatusClient(cfg config.Config) (client.VideoStatusClient, error) {
	if cfg.VideoServiceBaseURL == "" {
		return nil, nil
	}
	return client.NewHTTPVideoStatusClient(client.HTTPVideoStatusClientConfig{
		BaseURL:       cfg.VideoServiceBaseURL,
		InternalToken: cfg.InternalAPIToken,
		Timeout:       5 * time.Second,
	})
}

func startUploadedConsumer(ctx context.Context, cfg config.Config, processingService *service.ProcessingService, metrics *observability.Metrics, logger *slog.Logger) func() {
	if !cfg.ConsumerEnabled {
		logger.Info("uploaded event consumer disabled", "service", serviceName)
		return func() {}
	}
	consumer := event.NewKafkaConsumer(event.KafkaConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.VideoEventsTopic,
		GroupID: cfg.ConsumerGroup,
	})
	worker := event.NewUploadedConsumerWorker(event.UploadedConsumerConfig{
		Consumer: consumer,
		Service:  processingService,
		Logger:   logger,
		Metrics:  metrics,
	})
	go worker.Run(ctx)
	return func() {
		if err := worker.Close(); err != nil {
			logger.Error("failed to close uploaded event consumer", "service", serviceName, "error", err)
		}
	}
}
