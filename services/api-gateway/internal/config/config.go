package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

type Route struct {
	Prefix string
	Target *url.URL
}

type Config struct {
	Port     string
	LogLevel slog.Level
	Routes   []Route
}

func Load() (Config, error) {
	routes, err := loadRoutes()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:     getenv("PORT", "8080"),
		LogLevel: parseLogLevel(getenv("LOG_LEVEL", "info")),
		Routes:   routes,
	}, nil
}

func loadRoutes() ([]Route, error) {
	rawRoutes := map[string]string{
		"/api/v1/auth/":          getenv("IDENTITY_SERVICE_URL", "http://localhost:8081"),
		"/api/v1/users/":         getenv("IDENTITY_SERVICE_URL", "http://localhost:8081"),
		"/api/v1/videos/":        getenv("VIDEO_SERVICE_URL", "http://localhost:8082"),
		"/api/v1/feed":           getenv("FEED_SERVICE_URL", "http://localhost:8083"),
		"/api/v1/live-sessions/": getenv("LIVE_SERVICE_URL", "http://localhost:8084"),
		"/api/v1/incidents/":     getenv("AIOPS_SERVICE_URL", "http://localhost:8085"),
	}

	routes := make([]Route, 0, len(rawRoutes))
	for prefix, target := range rawRoutes {
		parsed, err := url.Parse(target)
		if err != nil {
			return nil, fmt.Errorf("parse route %s target %q: %w", prefix, target, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("route %s target must include scheme and host", prefix)
		}
		routes = append(routes, Route{Prefix: prefix, Target: parsed})
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

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
