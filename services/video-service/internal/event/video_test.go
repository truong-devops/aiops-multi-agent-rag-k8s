package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

func TestNewVideoUploadedOutbox(t *testing.T) {
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	event, err := NewVideoUploadedOutbox(domain.Video{
		ID:                "vid_123",
		OwnerID:           "usr_123",
		RawObjectKey:      "raw/vid_123/source.mp4",
		ContentType:       "video/mp4",
		SizeBytes:         2048,
		LastRequestID:     "req_123",
		LastCorrelationID: "corr_123",
	}, "dev", now)
	if err != nil {
		t.Fatalf("NewVideoUploadedOutbox() error = %v", err)
	}
	if event.EventName != VideoUploadedName || event.EventVersion != VideoUploadedVersion {
		t.Fatalf("event name/version = %s/%s", event.EventName, event.EventVersion)
	}
	if event.Status != domain.OutboxStatusPending {
		t.Fatalf("status = %q, want pending", event.Status)
	}
	if event.Producer != ProducerVideoService || event.Environment != "dev" {
		t.Fatalf("producer/environment = %s/%s", event.Producer, event.Environment)
	}
	if event.RequestID != "req_123" || event.CorrelationID != "corr_123" {
		t.Fatalf("request/correlation = %s/%s", event.RequestID, event.CorrelationID)
	}

	var payload VideoUploadedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.VideoID != "vid_123" || payload.OwnerID != "usr_123" || payload.RawObjectKey == "" {
		t.Fatalf("payload = %#v", payload)
	}
}
