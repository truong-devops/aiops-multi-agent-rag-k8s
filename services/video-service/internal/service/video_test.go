package service

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
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
	}); err == nil {
		t.Fatal("UpdateStatus() error = nil, want invalid transition")
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
