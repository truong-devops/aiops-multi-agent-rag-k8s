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
	RawVideoBucket        string
	UploadURLBase         string
	UploadRequestTTL      time.Duration
	RequestBodyLimitBytes int64
}

func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		Environment:           getenv("ENVIRONMENT", "local"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RawVideoBucket:        getenv("RAW_VIDEO_BUCKET", "raw-videos"),
		UploadURLBase:         strings.TrimRight(getenv("UPLOAD_URL_BASE", ""), "/"),
		UploadRequestTTL:      parseDuration(getenv("UPLOAD_REQUEST_TTL", "30m"), 30*time.Minute),
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
	if strings.TrimSpace(c.RawVideoBucket) == "" {
		return fmt.Errorf("RAW_VIDEO_BUCKET is required")
	}
	if c.UploadRequestTTL <= 0 {
		return fmt.Errorf("UPLOAD_REQUEST_TTL must be positive")
	}
	if c.RequestBodyLimitBytes <= 0 {
		return fmt.Errorf("REQUEST_BODY_LIMIT_BYTES must be positive")
	}
	if !c.IsLocal() && strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when ENVIRONMENT=%s", c.Environment)
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

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
