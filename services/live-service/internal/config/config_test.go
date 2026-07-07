package config

import "testing"

func TestLoadDefaultsForLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.LiveDefaultLimit != 20 || cfg.LiveMaxLimit != 50 {
		t.Fatalf("limits = %d/%d, want 20/50", cfg.LiveDefaultLimit, cfg.LiveMaxLimit)
	}
	if cfg.IngestBaseURL == "" || cfg.PlaybackBaseURL == "" {
		t.Fatalf("live URLs should have local defaults")
	}
}

func TestLoadRejectsMissingDatabaseOutsideLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
}

func TestLoadRejectsInvalidLimits(t *testing.T) {
	t.Setenv("LIVE_DEFAULT_LIMIT", "100")
	t.Setenv("LIVE_MAX_LIMIT", "50")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid limit error")
	}
}
