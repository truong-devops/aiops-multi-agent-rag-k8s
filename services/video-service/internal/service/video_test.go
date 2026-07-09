package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/event"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/storage"
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

func TestConfirmUploadedVerifiesObjectMetadata(t *testing.T) {
	store := repository.NewMemoryStore()
	verifier := &fakeObjectVerifier{
		metadata: storage.ObjectMetadata{SizeBytes: 2048, ContentType: "video/mp4"},
	}
	service := NewVideoService(store, Options{
		RawVideoBucket: "raw-videos",
		ObjectVerifier: verifier,
	})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:     "usr_123",
		Title:       "Launch video",
		Visibility:  domain.VisibilityPublic,
		ContentType: "video/mp4",
		Actor:       Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}

	uploaded, err := service.ConfirmUploaded(context.Background(), ConfirmUploadedInput{
		VideoID:         intent.Video.ID,
		UploadRequestID: intent.UploadRequest.ID,
		Actor:           Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("ConfirmUploaded() error = %v", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.calls)
	}
	if uploaded.SizeBytes != 2048 {
		t.Fatalf("uploaded size = %d, want metadata size 2048", uploaded.SizeBytes)
	}
}

func TestConfirmUploadedRejectsObjectMetadataMismatch(t *testing.T) {
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{
		RawVideoBucket: "raw-videos",
		ObjectVerifier: &fakeObjectVerifier{
			metadata: storage.ObjectMetadata{SizeBytes: 2048, ContentType: "video/mp4"},
		},
	})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:     "usr_123",
		Title:       "Launch video",
		Visibility:  domain.VisibilityPublic,
		ContentType: "video/mp4",
		Actor:       Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}

	if _, err := service.ConfirmUploaded(context.Background(), ConfirmUploadedInput{
		VideoID:         intent.Video.ID,
		UploadRequestID: intent.UploadRequest.ID,
		SizeBytes:       1024,
		Actor:           Actor{UserID: "usr_123"},
	}); err == nil {
		t.Fatal("ConfirmUploaded() error = nil, want metadata mismatch")
	}
}

func TestInternalActorCanDriveProcessingStatusFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	service := NewVideoService(store, Options{RawVideoBucket: "raw-videos"})

	intent, err := service.CreateUploadRequest(context.Background(), CreateUploadRequestInput{
		OwnerID:     "usr_123",
		Title:       "Launch video",
		Visibility:  domain.VisibilityPublic,
		ContentType: "video/mp4",
		Actor:       Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("CreateUploadRequest() error = %v", err)
	}
	uploaded, err := service.ConfirmUploaded(context.Background(), ConfirmUploadedInput{
		VideoID:         intent.Video.ID,
		UploadRequestID: intent.UploadRequest.ID,
		Actor:           Actor{UserID: "usr_123"},
	})
	if err != nil {
		t.Fatalf("ConfirmUploaded() error = %v", err)
	}
	processing, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		VideoID: uploaded.ID,
		Status:  domain.VideoStatusProcessing,
		Reason:  "worker_started",
		Actor:   Actor{Internal: true},
	})
	if err != nil {
		t.Fatalf("UpdateStatus(processing) error = %v", err)
	}
	ready, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		VideoID:            processing.ID,
		Status:             domain.VideoStatusReady,
		Reason:             "worker_completed",
		ProcessedObjectKey: "processed/" + processing.ID + "/source.mp4",
		ThumbnailObjectKey: "thumbnails/" + processing.ID + "/poster.jpg",
		DurationMs:         42000,
		Actor:              Actor{Internal: true},
	})
	if err != nil {
		t.Fatalf("UpdateStatus(ready) error = %v", err)
	}
	if ready.Status != domain.VideoStatusReady {
		t.Fatalf("ready status = %q", ready.Status)
	}
	events := store.OutboxEvents()
	if len(events) != 2 {
		t.Fatalf("outbox events = %d, want 2", len(events))
	}
	if events[1].EventName != event.VideoReadyName || events[1].EventVersion != event.VideoReadyVersion {
		t.Fatalf("ready event = %s/%s", events[1].EventName, events[1].EventVersion)
	}
	var payload event.VideoReadyPayload
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("unmarshal ready payload: %v", err)
	}
	if payload.VideoID != ready.ID || payload.OwnerID != "usr_123" || payload.ProcessedObjectKey == "" {
		t.Fatalf("ready payload = %#v", payload)
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

type fakeObjectVerifier struct {
	metadata storage.ObjectMetadata
	err      error
	calls    int
}

func (f *fakeObjectVerifier) VerifyObject(_ context.Context, _ storage.VerifyObjectInput) (storage.ObjectMetadata, error) {
	f.calls++
	return f.metadata, f.err
}
