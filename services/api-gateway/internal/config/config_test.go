package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadReadsGatewayConfigFromEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("IDENTITY_SERVICE_URL", "http://identity.internal:8080")
	t.Setenv("VIDEO_SERVICE_URL", "http://video.internal:8080")
	t.Setenv("FEED_SERVICE_URL", "http://feed.internal:8080")
	t.Setenv("LIVE_SERVICE_URL", "http://live.internal:8080")
	t.Setenv("AIOPS_SERVICE_URL", "http://aiops.internal:8080")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, http://localhost:5173")
	t.Setenv("REQUEST_BODY_LIMIT_BYTES", "2048")
	t.Setenv("UPSTREAM_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != "9090" {
		t.Fatalf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.RequestBodyLimitBytes != 2048 {
		t.Fatalf("RequestBodyLimitBytes = %d, want 2048", cfg.RequestBodyLimitBytes)
	}
	if cfg.UpstreamTimeout != 2*time.Second {
		t.Fatalf("UpstreamTimeout = %v, want 2s", cfg.UpstreamTimeout)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("CORSAllowedOrigins len = %d, want 2", len(cfg.CORSAllowedOrigins))
	}
	if len(cfg.Routes) != 6 {
		t.Fatalf("Routes len = %d, want 6", len(cfg.Routes))
	}

	route := cfg.Routes[0]
	if route.Name != "identity-service" || route.Prefix != "/api/v1/auth/" || route.Target.String() != "http://identity.internal:8080" {
		t.Fatalf("first route = %#v", route)
	}
}

func TestLoadRejectsInvalidRouteURL(t *testing.T) {
	t.Setenv("IDENTITY_SERVICE_URL", "identity-service:8080")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid URL error")
	}
}
