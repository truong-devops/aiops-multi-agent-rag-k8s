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
	InternalAPIToken      string
	RawVideoBucket        string
	UploadURLBase         string
	UploadRequestTTL      time.Duration
	PresignedUploadTTL    time.Duration
	RequestBodyLimitBytes int64
	MinIOEndpoint         string
	MinIOAccessKey        string
	MinIOSecretKey        string
	MinIORegion           string
	MinIOUseSSL           bool
	VerifyUploadObject    bool
	KafkaBrokers          []string
	VideoEventsTopic      string
	OutboxPublisher       bool
	OutboxPollInterval    time.Duration
	OutboxBatchSize       int
}

func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		Environment:           getenv("ENVIRONMENT", "local"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		InternalAPIToken:      strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN")),
		RawVideoBucket:        getenv("RAW_VIDEO_BUCKET", "raw-videos"),
		UploadURLBase:         strings.TrimRight(getenv("UPLOAD_URL_BASE", ""), "/"),
		UploadRequestTTL:      parseDuration(getenv("UPLOAD_REQUEST_TTL", "30m"), 30*time.Minute),
		PresignedUploadTTL:    parseDuration(getenv("PRESIGNED_UPLOAD_TTL", "15m"), 15*time.Minute),
		RequestBodyLimitBytes: parseInt64(getenv("REQUEST_BODY_LIMIT_BYTES", "1048576"), 1048576),
		MinIOEndpoint:         strings.TrimSpace(os.Getenv("MINIO_ENDPOINT")),
		MinIOAccessKey:        strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY")),
		MinIOSecretKey:        strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY")),
		MinIORegion:           getenv("MINIO_REGION", "us-east-1"),
		MinIOUseSSL:           parseBool(getenv("MINIO_USE_SSL", "false")),
		VerifyUploadObject:    parseBool(getenv("VERIFY_UPLOAD_OBJECT", "false")),
		KafkaBrokers:          parseCSV(os.Getenv("KAFKA_BROKERS")),
		VideoEventsTopic:      getenv("VIDEO_EVENTS_TOPIC", "video.events"),
		OutboxPublisher:       parseBool(getenv("OUTBOX_PUBLISHER_ENABLED", "false")),
		OutboxPollInterval:    parseDuration(getenv("OUTBOX_POLL_INTERVAL", "5s"), 5*time.Second),
		OutboxBatchSize:       parseInt(getenv("OUTBOX_BATCH_SIZE", "25"), 25),
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
	if strings.TrimSpace(c.RawVideoBucket) == "" {
		return fmt.Errorf("RAW_VIDEO_BUCKET is required")
	}
	if c.UploadRequestTTL <= 0 {
		return fmt.Errorf("UPLOAD_REQUEST_TTL must be positive")
	}
	if c.PresignedUploadTTL <= 0 {
		return fmt.Errorf("PRESIGNED_UPLOAD_TTL must be positive")
	}
	if c.RequestBodyLimitBytes <= 0 {
		return fmt.Errorf("REQUEST_BODY_LIMIT_BYTES must be positive")
	}
	if c.OutboxPublisher {
		if len(c.KafkaBrokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required when OUTBOX_PUBLISHER_ENABLED=true")
		}
		if strings.TrimSpace(c.VideoEventsTopic) == "" {
			return fmt.Errorf("VIDEO_EVENTS_TOPIC is required when OUTBOX_PUBLISHER_ENABLED=true")
		}
		if c.OutboxPollInterval <= 0 {
			return fmt.Errorf("OUTBOX_POLL_INTERVAL must be positive")
		}
		if c.OutboxBatchSize <= 0 {
			return fmt.Errorf("OUTBOX_BATCH_SIZE must be positive")
		}
	}
	if c.UseMinIOPresigner() && (strings.TrimSpace(c.MinIOAccessKey) == "" || strings.TrimSpace(c.MinIOSecretKey) == "") {
		return fmt.Errorf("MINIO_ACCESS_KEY and MINIO_SECRET_KEY are required when MINIO_ENDPOINT is set")
	}
	if c.VerifyUploadObject && !c.UseMinIOPresigner() {
		return fmt.Errorf("MINIO_ENDPOINT is required when VERIFY_UPLOAD_OBJECT=true")
	}
	if !c.IsLocal() && strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when ENVIRONMENT=%s", c.Environment)
	}
	if !c.IsLocal() && strings.TrimSpace(c.InternalAPIToken) == "" {
		return fmt.Errorf("INTERNAL_API_TOKEN is required when ENVIRONMENT=%s", c.Environment)
	}
	return nil
}

func (c Config) UsePostgres() bool {
	return strings.TrimSpace(c.DatabaseURL) != ""
}

func (c Config) UseMinIOPresigner() bool {
	return strings.TrimSpace(c.MinIOEndpoint) != ""
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

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
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

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
