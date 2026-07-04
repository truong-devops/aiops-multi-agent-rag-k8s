package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type DBMetrics interface {
	RecordDBOperation(operation string, outcome string, duration time.Duration)
}

type InstrumentedStore struct {
	next    Store
	metrics DBMetrics
	now     func() time.Time
}

func NewInstrumentedStore(next Store, metrics DBMetrics) Store {
	return InstrumentedStore{next: next, metrics: metrics, now: time.Now}
}

func (s InstrumentedStore) CreateJobFromUploadedEvent(ctx context.Context, event domain.UploadedVideoEvent, job domain.ProcessingJob) (domain.ProcessingJob, bool, error) {
	startedAt := s.now()
	out, created, err := s.next.CreateJobFromUploadedEvent(ctx, event, job)
	s.record("create_job_from_uploaded_event", err, startedAt)
	return out, created, err
}

func (s InstrumentedStore) FindJobByID(ctx context.Context, id string) (domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.FindJobByID(ctx, id)
	s.record("find_job_by_id", err, startedAt)
	return out, err
}

func (s InstrumentedStore) FindJobByVideoID(ctx context.Context, videoID string) (domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.FindJobByVideoID(ctx, videoID)
	s.record("find_job_by_video_id", err, startedAt)
	return out, err
}

func (s InstrumentedStore) ListJobs(ctx context.Context, filter ListJobsFilter) ([]domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.ListJobs(ctx, filter)
	s.record("list_jobs", err, startedAt)
	return out, err
}

func (s InstrumentedStore) ClaimRunnableJobs(ctx context.Context, workerID string, now time.Time, leaseTTL time.Duration, limit int) ([]domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.ClaimRunnableJobs(ctx, workerID, now, leaseTTL, limit)
	s.record("claim_runnable_jobs", err, startedAt)
	return out, err
}

func (s InstrumentedStore) StartAttempt(ctx context.Context, jobID string, workerID string, now time.Time) (domain.ProcessingJob, domain.ProcessingAttempt, error) {
	startedAt := s.now()
	job, attempt, err := s.next.StartAttempt(ctx, jobID, workerID, now)
	s.record("start_attempt", err, startedAt)
	return job, attempt, err
}

func (s InstrumentedStore) MarkAttemptSucceeded(ctx context.Context, jobID string, attemptID string, now time.Time, metrics []byte) (domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.MarkAttemptSucceeded(ctx, jobID, attemptID, now, metrics)
	s.record("mark_attempt_succeeded", err, startedAt)
	return out, err
}

func (s InstrumentedStore) MarkAttemptFailed(ctx context.Context, jobID string, attemptID string, now time.Time, errorCode string, errorMessage string, retryAt *time.Time, deadLetter *domain.DeadLetter) (domain.ProcessingJob, error) {
	startedAt := s.now()
	out, err := s.next.MarkAttemptFailed(ctx, jobID, attemptID, now, errorCode, errorMessage, retryAt, deadLetter)
	s.record("mark_attempt_failed", err, startedAt)
	return out, err
}

func (s InstrumentedStore) Stats(ctx context.Context, now time.Time) (StoreStats, error) {
	startedAt := s.now()
	out, err := s.next.Stats(ctx, now)
	s.record("stats", err, startedAt)
	return out, err
}

func (s InstrumentedStore) Ping(ctx context.Context) error {
	startedAt := s.now()
	err := s.next.Ping(ctx)
	s.record("ping", err, startedAt)
	return err
}

func (s InstrumentedStore) record(operation string, err error, startedAt time.Time) {
	if s.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	s.metrics.RecordDBOperation(operation, outcome, s.now().Sub(startedAt))
}
