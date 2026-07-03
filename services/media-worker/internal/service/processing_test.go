package service

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/repository"
)

func TestRegisterUploadedEventCreatesJob(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	svc := NewProcessingService(repository.NewMemoryStore(), Options{
		RawBucket:   "raw-videos",
		MaxAttempts: 3,
		Now:         func() time.Time { return now },
	})

	job, created, err := svc.RegisterUploadedEvent(context.Background(), domain.UploadedVideoEvent{
		EventID:       "evt_123",
		VideoID:       "vid_123",
		OwnerID:       "usr_123",
		RawObjectKey:  "raw/vid_123/source.mp4",
		ContentType:   "video/mp4",
		SizeBytes:     1024,
		RequestID:     "req_123",
		CorrelationID: "corr_123",
	})
	if err != nil {
		t.Fatalf("RegisterUploadedEvent() error = %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if job.Status != domain.JobStatusQueued || job.NextRunAt != now {
		t.Fatalf("job = %#v", job)
	}
}

func TestRegisterUploadedEventReusesDuplicate(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	svc := NewProcessingService(repository.NewMemoryStore(), Options{
		RawBucket: "raw-videos",
		Now:       func() time.Time { return now },
	})
	event := domain.UploadedVideoEvent{
		EventID:      "evt_123",
		VideoID:      "vid_123",
		RawObjectKey: "raw/vid_123/source.mp4",
		ReceivedAt:   now,
	}
	first, created, err := svc.RegisterUploadedEvent(context.Background(), event)
	if err != nil || !created {
		t.Fatalf("first RegisterUploadedEvent() = %#v/%v/%v", first, created, err)
	}
	second, created, err := svc.RegisterUploadedEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("second RegisterUploadedEvent() error = %v", err)
	}
	if created {
		t.Fatal("created duplicate = true, want false")
	}
	if second.ID != first.ID {
		t.Fatalf("second job id = %s, want %s", second.ID, first.ID)
	}
}
