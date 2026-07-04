package repository

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

func TestMemoryStoreCreateJobFromUploadedEventIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	event := uploadedEvent(now)
	job, err := domain.NewProcessingJobFromUploadedEvent(event, "raw-videos", 3, now)
	if err != nil {
		t.Fatalf("NewProcessingJobFromUploadedEvent() error = %v", err)
	}

	created, ok, err := store.CreateJobFromUploadedEvent(context.Background(), event, job)
	if err != nil {
		t.Fatalf("CreateJobFromUploadedEvent() error = %v", err)
	}
	if !ok {
		t.Fatal("created = false, want true")
	}
	again, ok, err := store.CreateJobFromUploadedEvent(context.Background(), event, job)
	if err != nil {
		t.Fatalf("CreateJobFromUploadedEvent(duplicate) error = %v", err)
	}
	if ok {
		t.Fatal("created duplicate = true, want false")
	}
	if again.ID != created.ID {
		t.Fatalf("duplicate job id = %s, want %s", again.ID, created.ID)
	}
}

func TestMemoryStoreClaimAndAttempts(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	event := uploadedEvent(now)
	job, err := domain.NewProcessingJobFromUploadedEvent(event, "raw-videos", 3, now)
	if err != nil {
		t.Fatalf("NewProcessingJobFromUploadedEvent() error = %v", err)
	}
	created, _, err := store.CreateJobFromUploadedEvent(context.Background(), event, job)
	if err != nil {
		t.Fatalf("CreateJobFromUploadedEvent() error = %v", err)
	}

	claimed, err := store.ClaimRunnableJobs(context.Background(), "worker-a", now, time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimRunnableJobs() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != created.ID || claimed[0].LockedBy != "worker-a" {
		t.Fatalf("claimed = %#v", claimed)
	}

	running, attempt, err := store.StartAttempt(context.Background(), created.ID, "worker-a", now.Add(time.Second))
	if err != nil {
		t.Fatalf("StartAttempt() error = %v", err)
	}
	if running.Status != domain.JobStatusRunning || attempt.AttemptNo != 1 {
		t.Fatalf("running=%#v attempt=%#v", running, attempt)
	}

	succeeded, err := store.MarkAttemptSucceeded(context.Background(), created.ID, attempt.ID, now.Add(2*time.Second), []byte(`{"duration_ms":10}`))
	if err != nil {
		t.Fatalf("MarkAttemptSucceeded() error = %v", err)
	}
	if succeeded.Status != domain.JobStatusSucceeded || succeeded.CompletedAt == nil {
		t.Fatalf("succeeded = %#v", succeeded)
	}
}

func TestMemoryStoreStats(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	event := uploadedEvent(now.Add(-2 * time.Minute))
	job, err := domain.NewProcessingJobFromUploadedEvent(event, "raw-videos", 3, now.Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("NewProcessingJobFromUploadedEvent() error = %v", err)
	}
	if _, _, err := store.CreateJobFromUploadedEvent(context.Background(), event, job); err != nil {
		t.Fatalf("CreateJobFromUploadedEvent() error = %v", err)
	}

	stats, err := store.Stats(context.Background(), now)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.JobStatusCounts[domain.JobStatusQueued] != 1 {
		t.Fatalf("status counts = %#v", stats.JobStatusCounts)
	}
	if stats.RunnableCount != 1 {
		t.Fatalf("RunnableCount = %d", stats.RunnableCount)
	}
	if stats.OldestRunnableAge != 2*time.Minute {
		t.Fatalf("OldestRunnableAge = %s", stats.OldestRunnableAge)
	}
}

func uploadedEvent(now time.Time) domain.UploadedVideoEvent {
	return domain.UploadedVideoEvent{
		EventID:       "evt_123",
		VideoID:       "vid_123",
		OwnerID:       "usr_123",
		RawObjectKey:  "raw/vid_123/source.mp4",
		ContentType:   "video/mp4",
		SizeBytes:     1024,
		RequestID:     "req_123",
		CorrelationID: "corr_123",
		ReceivedAt:    now,
	}
}
