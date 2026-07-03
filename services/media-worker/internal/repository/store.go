package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type ListJobsFilter struct {
	VideoID string
	Status  string
	Limit   int
}

type Store interface {
	CreateJobFromUploadedEvent(ctx context.Context, event domain.UploadedVideoEvent, job domain.ProcessingJob) (domain.ProcessingJob, bool, error)
	FindJobByID(ctx context.Context, id string) (domain.ProcessingJob, error)
	FindJobByVideoID(ctx context.Context, videoID string) (domain.ProcessingJob, error)
	ListJobs(ctx context.Context, filter ListJobsFilter) ([]domain.ProcessingJob, error)
	ClaimRunnableJobs(ctx context.Context, workerID string, now time.Time, leaseTTL time.Duration, limit int) ([]domain.ProcessingJob, error)
	StartAttempt(ctx context.Context, jobID string, workerID string, now time.Time) (domain.ProcessingJob, domain.ProcessingAttempt, error)
	MarkAttemptSucceeded(ctx context.Context, jobID string, attemptID string, now time.Time, metrics []byte) (domain.ProcessingJob, error)
	MarkAttemptFailed(ctx context.Context, jobID string, attemptID string, now time.Time, errorCode string, errorMessage string, retryAt *time.Time, deadLetter *domain.DeadLetter) (domain.ProcessingJob, error)
	Ping(ctx context.Context) error
}
