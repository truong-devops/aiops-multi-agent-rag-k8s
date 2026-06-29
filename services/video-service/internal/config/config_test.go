package config

import (
	"testing"
	"time"
)

func TestValidateAllowsLocalWithoutDatabase(t *testing.T) {
	cfg := validConfig("local")
	cfg.DatabaseURL = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresProductionDatabase(t *testing.T) {
	cfg := validConfig("production")
	cfg.DatabaseURL = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing DATABASE_URL error")
	}
}

func TestUsePostgres(t *testing.T) {
	cfg := validConfig("local")

	if !cfg.UsePostgres() {
		t.Fatal("UsePostgres() = false, want true")
	}

	cfg.DatabaseURL = ""
	if cfg.UsePostgres() {
		t.Fatal("UsePostgres() = true, want false")
	}
}

func validConfig(environment string) Config {
	return Config{
		Port:                  "8080",
		Environment:           environment,
		DatabaseURL:           "postgres://video:video@postgres:5432/video_db?sslmode=disable",
		RawVideoBucket:        "raw-videos",
		UploadRequestTTL:      30 * time.Minute,
		RequestBodyLimitBytes: 1048576,
	}
}
