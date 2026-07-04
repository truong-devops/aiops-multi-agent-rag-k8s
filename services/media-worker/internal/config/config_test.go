package config

import (
	"testing"
	"time"
)

func TestValidateAllowsLocalWithoutDependencies(t *testing.T) {
	cfg := validConfig("local")
	cfg.DatabaseURL = ""
	cfg.KafkaBrokers = nil
	cfg.MinIOEndpoint = ""
	cfg.VideoServiceBaseURL = ""
	cfg.InternalAPIToken = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresProductionDependencies(t *testing.T) {
	cfg := validConfig("production")
	cfg.DatabaseURL = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing DATABASE_URL error")
	}
}

func TestValidateRequiresKafkaWhenConsumerEnabled(t *testing.T) {
	cfg := validConfig("local")
	cfg.ConsumerEnabled = true
	cfg.KafkaBrokers = nil

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing KAFKA_BROKERS error")
	}
}

func TestValidateRequiresVideoServiceWhenRunnerEnabled(t *testing.T) {
	cfg := validConfig("local")
	cfg.RunnerEnabled = true
	cfg.VideoServiceBaseURL = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing VIDEO_SERVICE_BASE_URL error")
	}
}

func TestValidateRequiresFFmpegPathsInFFmpegMode(t *testing.T) {
	cfg := validConfig("local")
	cfg.ProcessingMode = "ffmpeg"
	cfg.FFmpegPath = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing FFMPEG_PATH error")
	}
}

func validConfig(environment string) Config {
	return Config{
		Port:                  "8080",
		Environment:           environment,
		DatabaseURL:           "postgres://media:media@postgres:5432/media_db?sslmode=disable",
		KafkaBrokers:          []string{"redpanda:9092"},
		VideoEventsTopic:      "video.events",
		MediaEventsTopic:      "media.events",
		ConsumerGroup:         "media-worker",
		WorkerID:              "worker-test",
		MaxAttempts:           3,
		JobLeaseTTL:           2 * time.Minute,
		JobPollInterval:       5 * time.Second,
		JobBatchSize:          10,
		MinIOEndpoint:         "minio:9000",
		MinIOAccessKey:        "minioadmin",
		MinIOSecretKey:        "minioadmin",
		RawVideoBucket:        "raw-videos",
		ProcessedVideoBucket:  "processed-videos",
		ThumbnailBucket:       "thumbnails",
		VideoServiceBaseURL:   "http://video-service:8080",
		InternalAPIToken:      "internal-secret",
		ProcessingMode:        "placeholder",
		FFmpegPath:            "ffmpeg",
		FFprobePath:           "ffprobe",
		ProcessingTimeout:     30 * time.Minute,
		RequestBodyLimitBytes: 1048576,
	}
}
