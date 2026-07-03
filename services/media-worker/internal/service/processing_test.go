package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/client"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/processor"
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

func TestRunOnceProcessesJobSuccessfully(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	statusClient := &fakeStatusClient{}
	svc := NewProcessingService(store, Options{
		RawBucket:    "raw-videos",
		MaxAttempts:  3,
		WorkerID:     "worker-test",
		LeaseTTL:     time.Minute,
		BatchSize:    10,
		StatusClient: statusClient,
		Processor:    fakeProcessor{result: processor.Result{Metrics: []byte(`{"ok":true}`)}},
		Now:          func() time.Time { return now },
	})
	_, _, err := svc.RegisterUploadedEvent(context.Background(), uploadedEvent(now))
	if err != nil {
		t.Fatalf("RegisterUploadedEvent() error = %v", err)
	}

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	job, err := store.FindJobByVideoID(context.Background(), "vid_123")
	if err != nil {
		t.Fatalf("FindJobByVideoID() error = %v", err)
	}
	if job.Status != domain.JobStatusSucceeded {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
	if got := statusClient.statuses; len(got) != 2 || got[0] != "processing" || got[1] != "ready" {
		t.Fatalf("statuses = %#v", got)
	}
}

func TestRunOnceSchedulesRetry(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	svc := NewProcessingService(store, Options{
		RawBucket:    "raw-videos",
		MaxAttempts:  3,
		WorkerID:     "worker-test",
		LeaseTTL:     time.Minute,
		BatchSize:    10,
		StatusClient: &fakeStatusClient{},
		Processor: fakeProcessor{err: domain.ProcessingError{
			Code:      domain.CodeMinIOUnavailable,
			Message:   "minio down",
			Retryable: true,
		}},
		Now: func() time.Time { return now },
	})
	_, _, err := svc.RegisterUploadedEvent(context.Background(), uploadedEvent(now))
	if err != nil {
		t.Fatalf("RegisterUploadedEvent() error = %v", err)
	}

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	job, err := store.FindJobByVideoID(context.Background(), "vid_123")
	if err != nil {
		t.Fatalf("FindJobByVideoID() error = %v", err)
	}
	if job.Status != domain.JobStatusRetrying {
		t.Fatalf("job status = %q, want retrying", job.Status)
	}
	if !job.NextRunAt.After(now) {
		t.Fatalf("next_run_at = %s, want after %s", job.NextRunAt, now)
	}
}

func TestRunOnceMovesToDeadLetter(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	store := repository.NewMemoryStore()
	statusClient := &fakeStatusClient{}
	svc := NewProcessingService(store, Options{
		RawBucket:    "raw-videos",
		MaxAttempts:  1,
		WorkerID:     "worker-test",
		LeaseTTL:     time.Minute,
		BatchSize:    10,
		StatusClient: statusClient,
		Processor: fakeProcessor{err: domain.ProcessingError{
			Code:      domain.CodeRawObjectNotFound,
			Message:   "missing raw object",
			Retryable: false,
		}},
		Now: func() time.Time { return now },
	})
	_, _, err := svc.RegisterUploadedEvent(context.Background(), uploadedEvent(now))
	if err != nil {
		t.Fatalf("RegisterUploadedEvent() error = %v", err)
	}

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	job, err := store.FindJobByVideoID(context.Background(), "vid_123")
	if err != nil {
		t.Fatalf("FindJobByVideoID() error = %v", err)
	}
	if job.Status != domain.JobStatusDeadLetter {
		t.Fatalf("job status = %q, want dead_letter", job.Status)
	}
	if got := statusClient.statuses; len(got) != 2 || got[1] != "failed" {
		t.Fatalf("statuses = %#v", got)
	}
}

func TestDecideRetry(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	job := domain.ProcessingJob{AttemptCount: 1, MaxAttempts: 3}
	decision := DecideRetry(job, domain.ProcessingError{Code: domain.CodeMinIOUnavailable, Message: "down", Retryable: true}, now)
	if decision.DeadLetter || decision.RetryAt == nil || !decision.RetryAt.After(now) {
		t.Fatalf("decision = %#v", decision)
	}

	job.AttemptCount = 3
	decision = DecideRetry(job, errors.New("boom"), now)
	if !decision.DeadLetter {
		t.Fatalf("decision = %#v, want dead letter", decision)
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

type fakeStatusClient struct {
	statuses []string
}

func (f *fakeStatusClient) UpdateStatus(_ context.Context, input client.UpdateVideoStatusInput) error {
	f.statuses = append(f.statuses, input.Status)
	return nil
}

type fakeProcessor struct {
	result processor.Result
	err    error
}

func (f fakeProcessor) Process(context.Context, domain.ProcessingJob, domain.ProcessingAttempt) (processor.Result, error) {
	return f.result, f.err
}
