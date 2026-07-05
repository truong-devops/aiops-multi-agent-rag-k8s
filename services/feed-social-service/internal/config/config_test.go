package config

import (
	"testing"
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

func TestValidateRequiresFeedDefaultLimitBelowMax(t *testing.T) {
	cfg := validConfig("local")
	cfg.FeedDefaultLimit = 100
	cfg.FeedMaxLimit = 50

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want limit error")
	}
}

func validConfig(environment string) Config {
	return Config{
		Port:                  "8080",
		Environment:           environment,
		DatabaseURL:           "postgres://feed:feed@postgres:5432/feed_social_db?sslmode=disable",
		RequestBodyLimitBytes: 1048576,
		FeedDefaultLimit:      20,
		FeedMaxLimit:          50,
		LogLevel:              parseLogLevel("info"),
	}
}
