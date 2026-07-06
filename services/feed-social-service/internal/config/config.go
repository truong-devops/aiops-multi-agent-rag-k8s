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
	KafkaBrokers          []string
	VideoEventsTopic      string
	ConsumerGroup         string
	ConsumerEnabled       bool
	InternalAPIToken      string
	RequestBodyLimitBytes int64
	FeedDefaultLimit      int
	FeedMaxLimit          int
}

func Load() (Config, error) {
	cfg := Config{
		Port:                  getenv("PORT", "8080"),
		Environment:           getenv("ENVIRONMENT", "local"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		KafkaBrokers:          parseCSV(os.Getenv("KAFKA_BROKERS")),
		VideoEventsTopic:      getenv("VIDEO_EVENTS_TOPIC", "video-events"),
		ConsumerGroup:         getenv("CONSUMER_GROUP", "feed-social-service"),
		ConsumerEnabled:       parseBool(getenv("CONSUMER_ENABLED", "false")),
		InternalAPIToken:      strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN")),
		RequestBodyLimitBytes: parseInt64(getenv("REQUEST_BODY_LIMIT_BYTES", "1048576"), 1048576),
		FeedDefaultLimit:      parseInt(getenv("FEED_DEFAULT_LIMIT", "20"), 20),
		FeedMaxLimit:          parseInt(getenv("FEED_MAX_LIMIT", "50"), 50),
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
	if c.FeedDefaultLimit <= 0 {
		return fmt.Errorf("FEED_DEFAULT_LIMIT must be positive")
	}
	if c.FeedMaxLimit <= 0 {
		return fmt.Errorf("FEED_MAX_LIMIT must be positive")
	}
	if c.FeedDefaultLimit > c.FeedMaxLimit {
		return fmt.Errorf("FEED_DEFAULT_LIMIT must be less than or equal to FEED_MAX_LIMIT")
	}
	if !c.IsLocal() && strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when ENVIRONMENT=%s", c.Environment)
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
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
