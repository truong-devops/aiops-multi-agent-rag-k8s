package config

import (
	"testing"
	"time"
)

func TestValidateAllowsLocalWithoutExternalSecrets(t *testing.T) {
	cfg := validConfig("local")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresProductionDatabaseRedisAndSigningKey(t *testing.T) {
	cfg := validConfig("production")
	cfg.DatabaseURL = ""
	cfg.RedisURL = ""
	cfg.SigningKeyPEM = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing DATABASE_URL error")
	}

	cfg.DatabaseURL = "postgres://identity:identity@postgres:5432/identity?sslmode=disable"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing SIGNING_KEY_PEM error")
	}

	cfg.SigningKeyPEM = "-----BEGIN RSA PRIVATE KEY-----\nplaceholder\n-----END RSA PRIVATE KEY-----"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing REDIS_URL error")
	}

	cfg.RedisURL = "redis://redis:6379/0"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAcceptsStagingWithRequiredExternalConfig(t *testing.T) {
	cfg := validConfig("staging")

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsStagingWithoutSigningKey(t *testing.T) {
	cfg := validConfig("staging")
	cfg.SigningKeyPEM = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing SIGNING_KEY_PEM error")
	}
}

func TestValidateRejectsInvalidTokenTTL(t *testing.T) {
	cfg := validConfig("production")
	cfg.AccessTokenTTL = cfg.RefreshTokenTTL

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want token ttl ordering error")
	}
}

func TestValidateRequiresGoogleRedirectAllowlistOutsideLocal(t *testing.T) {
	cfg := validConfig("production")
	cfg.GoogleClientID = "google-client"
	cfg.GoogleClientSecret = "google-secret"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing Google redirect allowlist error")
	}

	cfg.GoogleAllowedRedirectURIs = []string{"https://app.example.com/auth/google/callback"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsMissingIssuer(t *testing.T) {
	cfg := validConfig("local")
	cfg.Issuer = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing issuer error")
	}
}

func TestLoadPreservesInvalidDurationForValidation(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")
	t.Setenv("ACCESS_TOKEN_TTL", "not-a-duration")

	cfg := Load()
	if cfg.AccessTokenTTL != 0 {
		t.Fatalf("AccessTokenTTL = %s, want zero value for invalid env", cfg.AccessTokenTTL)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid access token ttl error")
	}
}

func TestLoadPreservesInvalidRateLimitForValidation(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")
	t.Setenv("LOGIN_RATE_LIMIT", "not-a-number")

	cfg := Load()
	if cfg.LoginRateLimit != 0 {
		t.Fatalf("LoginRateLimit = %d, want zero value for invalid env", cfg.LoginRateLimit)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid login rate limit error")
	}
}

func validConfig(environment string) Config {
	return Config{
		Environment:             environment,
		Issuer:                  "aiops-video-platform",
		Audience:                "aiops-api",
		DatabaseURL:             "postgres://identity:identity@postgres:5432/identity?sslmode=disable",
		RedisURL:                "redis://redis:6379/0",
		SigningKeyPEM:           "-----BEGIN RSA PRIVATE KEY-----\nplaceholder\n-----END RSA PRIVATE KEY-----",
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         7 * 24 * time.Hour,
		LoginRateLimit:          5,
		LoginRateLimitWindow:    15 * time.Minute,
		RegisterRateLimit:       10,
		RegisterRateLimitWindow: 15 * time.Minute,
	}
}
