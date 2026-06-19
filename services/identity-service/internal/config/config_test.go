package config

import "testing"

func TestValidateAllowsLocalWithoutExternalSecrets(t *testing.T) {
	cfg := Config{Environment: "local"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresProductionDatabaseURLAndSigningKey(t *testing.T) {
	cfg := Config{Environment: "production"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing DATABASE_URL error")
	}

	cfg.DatabaseURL = "postgres://identity:identity@postgres:5432/identity?sslmode=disable"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing SIGNING_KEY_PEM error")
	}

	cfg.SigningKeyPEM = "-----BEGIN RSA PRIVATE KEY-----\nplaceholder\n-----END RSA PRIVATE KEY-----"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
