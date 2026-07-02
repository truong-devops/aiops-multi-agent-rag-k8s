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

func TestValidateRequiresProductionInternalToken(t *testing.T) {
	cfg := validConfig("production")
	cfg.InternalAPIToken = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing INTERNAL_API_TOKEN error")
	}
}

func TestValidateRequiresMinIOCredentials(t *testing.T) {
	cfg := validConfig("local")
	cfg.MinIOEndpoint = "minio:9000"
	cfg.MinIOAccessKey = ""
	cfg.MinIOSecretKey = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing MinIO credentials error")
	}
}

func TestValidateRequiresMinIOWhenObjectVerificationEnabled(t *testing.T) {
	cfg := validConfig("local")
	cfg.VerifyUploadObject = true

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing MINIO_ENDPOINT error")
	}
}

func TestValidateRequiresKafkaWhenOutboxPublisherEnabled(t *testing.T) {
	cfg := validConfig("local")
	cfg.OutboxPublisher = true
	cfg.KafkaBrokers = nil

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing KAFKA_BROKERS error")
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
		InternalAPIToken:      "internal-secret",
		RawVideoBucket:        "raw-videos",
		UploadRequestTTL:      30 * time.Minute,
		PresignedUploadTTL:    15 * time.Minute,
		RequestBodyLimitBytes: 1048576,
		VideoEventsTopic:      "video.events",
		OutboxPollInterval:    5 * time.Second,
		OutboxBatchSize:       25,
	}
}
