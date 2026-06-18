package config

import (
	"log/slog"
	"os"
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
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	GoogleClientID     string
	GoogleClientSecret string
	GoogleAuthURL      string
	GoogleTokenURL     string
	GoogleJWKSURL      string
	GoogleScopes       []string
}

func Load() Config {
	return Config{
		Port:               getenv("PORT", "8080"),
		LogLevel:           parseLogLevel(getenv("LOG_LEVEL", "info")),
		Environment:        getenv("ENVIRONMENT", "local"),
		Issuer:             getenv("JWT_ISSUER", "aiops-video-platform"),
		Audience:           getenv("JWT_AUDIENCE", "aiops-api"),
		SigningKeyPEM:      os.Getenv("SIGNING_KEY_PEM"),
		AccessTokenTTL:     parseDuration(getenv("ACCESS_TOKEN_TTL", "15m"), 15*time.Minute),
		RefreshTokenTTL:    parseDuration(getenv("REFRESH_TOKEN_TTL", "168h"), 7*24*time.Hour),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAuthURL:      getenv("GOOGLE_AUTH_URL", "https://accounts.google.com/o/oauth2/v2/auth"),
		GoogleTokenURL:     getenv("GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token"),
		GoogleJWKSURL:      getenv("GOOGLE_JWKS_URL", "https://www.googleapis.com/oauth2/v3/certs"),
		GoogleScopes:       parseCSV(getenv("GOOGLE_SCOPES", "openid,email,profile")),
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
