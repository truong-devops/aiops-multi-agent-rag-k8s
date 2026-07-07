package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                  string
	Environment           string
	LogLevel              slog.Level
	DatabaseURL           string
	RequestBodyLimitBytes int64
	LiveDefaultLimit      int
	LiveMaxLimit          int
	IngestBaseURL         string
	PlaybackBaseURL       string
	StreamKeyBytes        int
}

func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		Environment:           getenv("ENVIRONMENT", "local"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RequestBodyLimitBytes: parseInt64(getenv("REQUEST_BODY_LIMIT_BYTES", "1048576"), 1048576),
		LiveDefaultLimit:      parseInt(getenv("LIVE_DEFAULT_LIMIT", "20"), 20),
		LiveMaxLimit:          parseInt(getenv("LIVE_MAX_LIMIT", "50"), 50),
		IngestBaseURL:         strings.TrimRight(getenv("LIVE_INGEST_BASE_URL", "rtmp://localhost:1935/live"), "/"),
		PlaybackBaseURL:       strings.TrimRight(getenv("LIVE_PLAYBACK_BASE_URL", "http://localhost:8888/live"), "/"),
		StreamKeyBytes:        parseInt(getenv("STREAM_KEY_BYTES", "32"), 32),
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
	if c.LiveDefaultLimit <= 0 {
		return fmt.Errorf("LIVE_DEFAULT_LIMIT must be positive")
	}
	if c.LiveMaxLimit <= 0 {
		return fmt.Errorf("LIVE_MAX_LIMIT must be positive")
	}
	if c.LiveDefaultLimit > c.LiveMaxLimit {
		return fmt.Errorf("LIVE_DEFAULT_LIMIT must be less than or equal to LIVE_MAX_LIMIT")
	}
	if strings.TrimSpace(c.IngestBaseURL) == "" {
		return fmt.Errorf("LIVE_INGEST_BASE_URL is required")
	}
	if strings.TrimSpace(c.PlaybackBaseURL) == "" {
		return fmt.Errorf("LIVE_PLAYBACK_BASE_URL is required")
	}
	if c.StreamKeyBytes < 24 {
		return fmt.Errorf("STREAM_KEY_BYTES must be at least 24")
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

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
