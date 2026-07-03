package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

func TestPostgresStoreProcessingJobFlow(t *testing.T) {
	databaseURL := os.Getenv("MEDIA_WORKER_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MEDIA_WORKER_TEST_DATABASE_URL to run postgres integration tests")
	}

	ctx := context.Background()
	store, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	applyMediaMigrations(t, store.db)
	truncateMediaTables(t, store.db)

	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	event := domain.UploadedVideoEvent{
		EventID:       "evt_pgtest_uploaded",
		VideoID:       "vid_pgtest",
		OwnerID:       "usr_pgtest",
		RawObjectKey:  "raw/vid_pgtest/source.mp4",
		ContentType:   "video/mp4",
		SizeBytes:     2048,
		RequestID:     "req_pgtest",
		CorrelationID: "corr_pgtest",
		ReceivedAt:    now,
	}
	job, err := domain.NewProcessingJobFromUploadedEvent(event, "raw-videos", 3, now)
	if err != nil {
		t.Fatalf("NewProcessingJobFromUploadedEvent() error = %v", err)
	}
	job.ID = "job_pgtest"

	created, ok, err := store.CreateJobFromUploadedEvent(ctx, event, job)
	if err != nil {
		t.Fatalf("CreateJobFromUploadedEvent() error = %v", err)
	}
	if !ok || created.ID != job.ID {
		t.Fatalf("created=%v job=%#v", ok, created)
	}
	again, ok, err := store.CreateJobFromUploadedEvent(ctx, event, job)
	if err != nil {
		t.Fatalf("CreateJobFromUploadedEvent(duplicate) error = %v", err)
	}
	if ok || again.ID != job.ID {
		t.Fatalf("duplicate created=%v job=%#v", ok, again)
	}

	claimed, err := store.ClaimRunnableJobs(ctx, "worker-pgtest", now, time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimRunnableJobs() error = %v", err)
	}
	if len(claimed) != 1 || claimed[0].LockedBy != "worker-pgtest" {
		t.Fatalf("claimed = %#v", claimed)
	}
	running, attempt, err := store.StartAttempt(ctx, job.ID, "worker-pgtest", now.Add(time.Second))
	if err != nil {
		t.Fatalf("StartAttempt() error = %v", err)
	}
	if running.Status != domain.JobStatusRunning || attempt.AttemptNo != 1 {
		t.Fatalf("running=%#v attempt=%#v", running, attempt)
	}
	succeeded, err := store.MarkAttemptSucceeded(ctx, job.ID, attempt.ID, now.Add(2*time.Second), []byte(`{"duration_ms":10}`))
	if err != nil {
		t.Fatalf("MarkAttemptSucceeded() error = %v", err)
	}
	if succeeded.Status != domain.JobStatusSucceeded || succeeded.CompletedAt == nil {
		t.Fatalf("succeeded = %#v", succeeded)
	}
}

func applyMediaMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, name := range []string{"001_processing_schema.sql"} {
		path := filepath.Join("..", "..", "migrations", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func truncateMediaTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		TRUNCATE TABLE
			dead_letters,
			processing_attempts,
			processing_jobs,
			inbox_events
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate media tables: %v", err)
	}
}
