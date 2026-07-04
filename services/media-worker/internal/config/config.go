package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                  string
	Environment           string
	LogLevel              slog.Level
	DatabaseURL           string
	KafkaBrokers          []string
	VideoEventsTopic      string
	MediaEventsTopic      string
	ConsumerGroup         string
	ConsumerEnabled       bool
	RunnerEnabled         bool
	WorkerID              string
	MaxAttempts           int
	JobLeaseTTL           time.Duration
	JobPollInterval       time.Duration
	JobBatchSize          int
	MinIOEndpoint         string
	MinIOAccessKey        string
	MinIOSecretKey        string
	MinIORegion           string
	MinIOUseSSL           bool
	RawVideoBucket        string
	ProcessedVideoBucket  string
	ThumbnailBucket       string
	VideoServiceBaseURL   string
	InternalAPIToken      string
	ProcessingMode        string
	FFmpegPath            string
	FFprobePath           string
	ProcessingTimeout     time.Duration
	RequestBodyLimitBytes int64
}

func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		Environment:           getenv("ENVIRONMENT", "local"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		KafkaBrokers:          parseCSV(os.Getenv("KAFKA_BROKERS")),
		VideoEventsTopic:      getenv("VIDEO_EVENTS_TOPIC", "video.events"),
		MediaEventsTopic:      getenv("MEDIA_EVENTS_TOPIC", "media.events"),
		ConsumerGroup:         getenv("CONSUMER_GROUP", "media-worker"),
		ConsumerEnabled:       parseBool(getenv("CONSUMER_ENABLED", "false")),
		RunnerEnabled:         parseBool(getenv("RUNNER_ENABLED", "false")),
		WorkerID:              getenv("WORKER_ID", hostnameFallback()),
		MaxAttempts:           parseInt(getenv("MAX_ATTEMPTS", "3"), 3),
		JobLeaseTTL:           parseDuration(getenv("JOB_LEASE_TTL", "2m"), 2*time.Minute),
		JobPollInterval:       parseDuration(getenv("JOB_POLL_INTERVAL", "5s"), 5*time.Second),
		JobBatchSize:          parseInt(getenv("JOB_BATCH_SIZE", "10"), 10),
		MinIOEndpoint:         strings.TrimSpace(os.Getenv("MINIO_ENDPOINT")),
		MinIOAccessKey:        strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY")),
		MinIOSecretKey:        strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY")),
		MinIORegion:           getenv("MINIO_REGION", "us-east-1"),
		MinIOUseSSL:           parseBool(getenv("MINIO_USE_SSL", "false")),
		RawVideoBucket:        getenv("RAW_VIDEO_BUCKET", "raw-videos"),
		ProcessedVideoBucket:  getenv("PROCESSED_VIDEO_BUCKET", "processed-videos"),
		ThumbnailBucket:       getenv("THUMBNAIL_BUCKET", "thumbnails"),
		VideoServiceBaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("VIDEO_SERVICE_BASE_URL")), "/"),
		InternalAPIToken:      strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN")),
		ProcessingMode:        getenv("PROCESSING_MODE", "placeholder"),
		FFmpegPath:            getenv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:           getenv("FFPROBE_PATH", "ffprobe"),
		ProcessingTimeout:     parseDuration(getenv("PROCESSING_TIMEOUT", "30m"), 30*time.Minute),
		RequestBodyLimitBytes: parseInt64(getenv("REQUEST_BODY_LIMIT_BYTES", "1048576"), 1048576),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.RequestBodyLimitBytes <= 0 {
		return fmt.Errorf("REQUEST_BODY_LIMIT_BYTES must be positive")
	}
	if strings.TrimSpace(c.WorkerID) == "" {
		return fmt.Errorf("WORKER_ID is required")
	}
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("MAX_ATTEMPTS must be positive")
	}
	if c.JobLeaseTTL <= 0 {
		return fmt.Errorf("JOB_LEASE_TTL must be positive")
	}
	if c.JobPollInterval <= 0 {
		return fmt.Errorf("JOB_POLL_INTERVAL must be positive")
	}
	if c.JobBatchSize <= 0 {
		return fmt.Errorf("JOB_BATCH_SIZE must be positive")
	}
	if !validProcessingMode(c.ProcessingMode) {
		return fmt.Errorf("PROCESSING_MODE must be placeholder or ffmpeg")
	}
	if c.ProcessingTimeout <= 0 {
		return fmt.Errorf("PROCESSING_TIMEOUT must be positive")
	}
	if c.ConsumerEnabled {
		if len(c.KafkaBrokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required when CONSUMER_ENABLED=true")
		}
		if strings.TrimSpace(c.VideoEventsTopic) == "" {
			return fmt.Errorf("VIDEO_EVENTS_TOPIC is required when CONSUMER_ENABLED=true")
		}
		if strings.TrimSpace(c.ConsumerGroup) == "" {
			return fmt.Errorf("CONSUMER_GROUP is required when CONSUMER_ENABLED=true")
		}
	}
	if c.RunnerEnabled {
		if strings.TrimSpace(c.DatabaseURL) == "" {
			return fmt.Errorf("DATABASE_URL is required when RUNNER_ENABLED=true")
		}
		if strings.TrimSpace(c.VideoServiceBaseURL) == "" {
			return fmt.Errorf("VIDEO_SERVICE_BASE_URL is required when RUNNER_ENABLED=true")
		}
		if strings.TrimSpace(c.InternalAPIToken) == "" {
			return fmt.Errorf("INTERNAL_API_TOKEN is required when RUNNER_ENABLED=true")
		}
	}
	if c.ProcessingMode == "ffmpeg" {
		if strings.TrimSpace(c.FFmpegPath) == "" {
			return fmt.Errorf("FFMPEG_PATH is required when PROCESSING_MODE=ffmpeg")
		}
		if strings.TrimSpace(c.FFprobePath) == "" {
			return fmt.Errorf("FFPROBE_PATH is required when PROCESSING_MODE=ffmpeg")
		}
	}
	if !c.IsLocal() {
		if strings.TrimSpace(c.DatabaseURL) == "" {
			return fmt.Errorf("DATABASE_URL is required when ENVIRONMENT=%s", c.Environment)
		}
		if len(c.KafkaBrokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required when ENVIRONMENT=%s", c.Environment)
		}
		if strings.TrimSpace(c.MinIOEndpoint) == "" {
			return fmt.Errorf("MINIO_ENDPOINT is required when ENVIRONMENT=%s", c.Environment)
		}
		if strings.TrimSpace(c.MinIOAccessKey) == "" || strings.TrimSpace(c.MinIOSecretKey) == "" {
			return fmt.Errorf("MINIO_ACCESS_KEY and MINIO_SECRET_KEY are required when ENVIRONMENT=%s", c.Environment)
		}
		if strings.TrimSpace(c.VideoServiceBaseURL) == "" {
			return fmt.Errorf("VIDEO_SERVICE_BASE_URL is required when ENVIRONMENT=%s", c.Environment)
		}
		if strings.TrimSpace(c.InternalAPIToken) == "" {
			return fmt.Errorf("INTERNAL_API_TOKEN is required when ENVIRONMENT=%s", c.Environment)
		}
	}
	return nil
}

func (c Config) UsePostgres() bool {
	return strings.TrimSpace(c.DatabaseURL) != ""
}

func (c Config) IsLocal() bool {
	switch strings.ToLower(strings.TrimSpace(c.Environment)) {
	case "", "local", "dev", "development", "test":
		return true
	default:
		return false
	}
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func validProcessingMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "placeholder", "ffmpeg":
		return true
	default:
		return false
	}
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func hostnameFallback() string {
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return hostname
	}
	return "media-worker-local"
}
