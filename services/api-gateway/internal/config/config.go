package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Route struct {
	Name   string
	Prefix string
	Target *url.URL
}

type Config struct {
	Port                  string
	LogLevel              slog.Level
	Routes                []Route
	CORSAllowedOrigins    []string
	RequestBodyLimitBytes int64
	UpstreamTimeout       time.Duration
}

func Load() (Config, error) {
	routes, err := loadRoutes()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:                  getenv("PORT", "8080"),
		LogLevel:              parseLogLevel(getenv("LOG_LEVEL", "info")),
		Routes:                routes,
		CORSAllowedOrigins:    parseCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		RequestBodyLimitBytes: parseInt64(getenv("REQUEST_BODY_LIMIT_BYTES", "1048576"), 1048576),
		UpstreamTimeout:       parseDuration(getenv("UPSTREAM_TIMEOUT", "15s"), 15*time.Second),
	}, nil
}

func loadRoutes() ([]Route, error) {
	rawRoutes := []struct {
		name   string
		prefix string
		target string
	}{
		{name: "identity-service", prefix: "/api/v1/auth/", target: getenv("IDENTITY_SERVICE_URL", "http://localhost:8081")},
		{name: "identity-service", prefix: "/api/v1/users/", target: getenv("IDENTITY_SERVICE_URL", "http://localhost:8081")},
		{name: "video-service", prefix: "/api/v1/videos/", target: getenv("VIDEO_SERVICE_URL", "http://localhost:8082")},
		{name: "feed-social-service", prefix: "/api/v1/feed", target: getenv("FEED_SERVICE_URL", "http://localhost:8083")},
		{name: "live-service", prefix: "/api/v1/live-sessions/", target: getenv("LIVE_SERVICE_URL", "http://localhost:8084")},
		{name: "aiops-service", prefix: "/api/v1/incidents/", target: getenv("AIOPS_SERVICE_URL", "http://localhost:8085")},
	}

	routes := make([]Route, 0, len(rawRoutes))
	for _, rawRoute := range rawRoutes {
		parsed, err := url.Parse(rawRoute.target)
		if err != nil {
			return nil, fmt.Errorf("parse route %s target %q: %w", rawRoute.prefix, rawRoute.target, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("route %s target must include scheme and host", rawRoute.prefix)
		}
		routes = append(routes, Route{Name: rawRoute.name, Prefix: rawRoute.prefix, Target: parsed})
	}
	return routes, nil
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

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func parseInt64(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
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
