package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/event"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
)

func TestCreateUploadRequestAndConfirmUploaded(t *testing.T) {
	now := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{
		RawVideoBucket:   "raw-videos",
		UploadURLBase:    "http://minio.local",
		UploadRequestTTL: time.Hour,
		Now:              func() time.Time { return now },
	})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:       "usr_123",
		Title:         "Launch video",
		Visibility:    domain.VisibilityPublic,
		ContentType:   "video/mp4",
		SizeBytes:     1024,
		RequestID:     "req_123",
		CorrelationID: "corr_123",
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}
	if intent.Video.Status != domain.VideoStatusDraft {
		t.Fatalf("video status = %q, want draft", intent.Video.Status)
	}
	if intent.UploadRequest.Status != domain.UploadStatusCreated {
		t.Fatalf("upload status = %q, want created", intent.UploadRequest.Status)
	}
	if intent.UploadURL != "http://minio.local/raw-videos/"+intent.UploadRequest.ObjectKey {
		t.Fatalf("upload url = %q", intent.UploadURL)
	}

	uploaded, err := service.ConfirmUploaded(context.Background(), ConfirmUploadedInput{
		UploadRequestID: intent.UploadRequest.ID,
		SizeBytes:       2048,
		Actor:           Actor{UserID: "usr_123"},
		RequestID:       "req_124",
		CorrelationID:   "corr_123",
	})
	if err != nil {
		t.Fatalf("ConfirmUploaded() error = %v", err)
	}
	if uploaded.Status != domain.VideoStatusUploaded {
		t.Fatalf("uploaded status = %q, want uploaded", uploaded.Status)
	}
	if uploaded.SizeBytes != 2048 {
		t.Fatalf("uploaded size = %d, want 2048", uploaded.SizeBytes)
	}
	events := store.OutboxEvents()
	if len(events) != 1 {
		t.Fatalf("outbox events = %d, want 1", len(events))
	}
	if events[0].EventName != event.VideoUploadedName || events[0].EventVersion != event.VideoUploadedVersion {
		t.Fatalf("outbox event name/version = %s/%s", events[0].EventName, events[0].EventVersion)
	}
	if events[0].Status != domain.OutboxStatusPending {
		t.Fatalf("outbox status = %q, want pending", events[0].Status)
	}
	if events[0].RequestID != "req_124" || events[0].CorrelationID != "corr_123" {
		t.Fatalf("outbox request/correlation = %s/%s", events[0].RequestID, events[0].CorrelationID)
	}
	var payload event.VideoUploadedPayload
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if payload.VideoID != uploaded.ID || payload.OwnerID != "usr_123" || payload.SizeBytes != 2048 {
		t.Fatalf("outbox payload = %#v", payload)
	}
}

func TestUpdateStatusRejectsInvalidTransition(t *testing.T) {
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{RawVideoBucket: "raw-videos"})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:     "usr_123",
		Title:       "Launch video",
		ContentType: "video/mp4",
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}

	if _, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		VideoID: intent.Video.ID,
		Status:  domain.VideoStatusReady,
		Actor:   Actor{Internal: true},
	}); err == nil {
		t.Fatal("UpdateStatus() error = nil, want invalid transition")
	}
}

func TestCreateUploadRequestReusesIdempotencyKey(t *testing.T) {
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{
		RawVideoBucket:   "raw-videos",
		UploadURLBase:    "http://minio.local",
		UploadRequestTTL: time.Hour,
	})

	input := CreateUploadRequestInput{
		OwnerID:        "usr_123",
		Title:          "Launch video",
		ContentType:    "video/mp4",
		IdempotencyKey: "idem_123",
		Actor:          Actor{UserID: "usr_123"},
	}
	first, err := service.CreateUploadRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("first CreateUploadRequest() error = %v", err)
	}
	second, err := service.CreateUploadRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("second CreateUploadRequest() error = %v", err)
	}
	if second.Video.ID != first.Video.ID || second.UploadRequest.ID != first.UploadRequest.ID {
		t.Fatalf("idempotent intent mismatch: first=%s/%s second=%s/%s", first.Video.ID, first.UploadRequest.ID, second.Video.ID, second.UploadRequest.ID)
	}
}

func TestOwnerCannotUpdateProcessingStatus(t *testing.T) {
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{RawVideoBucket: "raw-videos"})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:     "usr_123",
		Title:       "Launch video",
		ContentType: "video/mp4",
		Actor:       Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}

	if _, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		VideoID: intent.Video.ID,
		Status:  domain.VideoStatusProcessing,
		Actor:   Actor{UserID: "usr_123"},
	}); err == nil {
		t.Fatal("UpdateStatus() error = nil, want forbidden")
	}
}

func TestCreateUploadRequestRequiresOwner(t *testing.T) {
	service := NewVideoService(repository.NewMemoryStore(), Options{RawVideoBucket: "raw-videos"})

	if _, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		Title:       "Launch video",
		ContentType: "video/mp4",
	}); err == nil {
		t.Fatal("CreateUploadRequest() error = nil, want unauthorized")
	}
}
