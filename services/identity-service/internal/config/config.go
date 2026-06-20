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
	Port            string
	LogLevel        slog.Level
	Environment     string
	Issuer          string
	Audience        string
	SigningKeyPEM   string
	DatabaseURL     string
	RedisURL        string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	LoginRateLimit          int64
	LoginRateLimitWindow    time.Duration
	RegisterRateLimit       int64
	RegisterRateLimitWindow time.Duration

	GoogleClientID     string
	GoogleClientSecret string
	GoogleAuthURL      string
	GoogleTokenURL     string
	GoogleJWKSURL      string
	GoogleScopes       []string
}

func Load() Config {
	return Config{
		Port:            getenv("PORT", "8080"),
		LogLevel:        parseLogLevel(getenv("LOG_LEVEL", "info")),
		Environment:     getenv("ENVIRONMENT", "local"),
		Issuer:          getenv("JWT_ISSUER", "aiops-video-platform"),
		Audience:        getenv("JWT_AUDIENCE", "aiops-api"),
		SigningKeyPEM:   os.Getenv("SIGNING_KEY_PEM"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		AccessTokenTTL:  parseDuration(getenv("ACCESS_TOKEN_TTL", "15m"), 15*time.Minute),
		RefreshTokenTTL: parseDuration(getenv("REFRESH_TOKEN_TTL", "168h"), 7*24*time.Hour),
		LoginRateLimit:  parseInt64(getenv("LOGIN_RATE_LIMIT", "5"), 5),
		LoginRateLimitWindow: parseDuration(
			getenv("LOGIN_RATE_LIMIT_WINDOW", "15m"),
			15*time.Minute,
		),
		RegisterRateLimit: parseInt64(getenv("REGISTER_RATE_LIMIT", "10"), 10),
		RegisterRateLimitWindow: parseDuration(
			getenv("REGISTER_RATE_LIMIT_WINDOW", "15m"),
			15*time.Minute,
		),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAuthURL:      getenv("GOOGLE_AUTH_URL", "https://accounts.google.com/o/oauth2/v2/auth"),
		GoogleTokenURL:     getenv("GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token"),
		GoogleJWKSURL:      getenv("GOOGLE_JWKS_URL", "https://www.googleapis.com/oauth2/v3/certs"),
		GoogleScopes:       parseCSV(getenv("GOOGLE_SCOPES", "openid,email,profile")),
	}
}

func (c Config) Validate() error {
	if c.IsLocal() {
		return nil
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when ENVIRONMENT=%s", c.Environment)
	}
	if strings.TrimSpace(c.SigningKeyPEM) == "" {
		return fmt.Errorf("SIGNING_KEY_PEM is required when ENVIRONMENT=%s", c.Environment)
	}
	if strings.TrimSpace(c.RedisURL) == "" {
		return fmt.Errorf("REDIS_URL is required when ENVIRONMENT=%s", c.Environment)
	}
	return nil
}

func (c Config) UsePostgres() bool {
	return strings.TrimSpace(c.DatabaseURL) != ""
}

func (c Config) UseRedis() bool {
	return strings.TrimSpace(c.RedisURL) != ""
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

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
