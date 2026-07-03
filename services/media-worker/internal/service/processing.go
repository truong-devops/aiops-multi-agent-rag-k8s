package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/repository"
)

type ProcessingService struct {
	store       repository.Store
	rawBucket   string
	maxAttempts int
	logger      *slog.Logger
	now         func() time.Time
}

type Options struct {
	RawBucket   string
	MaxAttempts int
	Logger      *slog.Logger
	Now         func() time.Time
}

func NewProcessingService(store repository.Store, options Options) *ProcessingService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &ProcessingService{
		store:       store,
		rawBucket:   options.RawBucket,
		maxAttempts: maxAttempts,
		logger:      logger,
		now:         now,
	}
}

func (s *ProcessingService) RegisterUploadedEvent(ctx context.Context, event domain.UploadedVideoEvent) (domain.ProcessingJob, bool, error) {
	now := s.now().UTC()
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = now
	}
	job, err := domain.NewProcessingJobFromUploadedEvent(event, s.rawBucket, s.maxAttempts, now)
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	createdJob, created, err := s.store.CreateJobFromUploadedEvent(ctx, event, job)
	if err != nil {
		return domain.ProcessingJob{}, false, err
	}
	if created {
		s.logger.Info(
			"processing job created",
			"job_id", createdJob.ID,
			"video_id", createdJob.VideoID,
			"request_id", createdJob.RequestID,
			"correlation_id", createdJob.CorrelationID,
		)
	} else {
		s.logger.Info(
			"processing job reused for duplicate event",
			"job_id", createdJob.ID,
			"video_id", createdJob.VideoID,
			"event_id", event.EventID,
			"request_id", event.RequestID,
			"correlation_id", event.CorrelationID,
		)
	}
	return createdJob, created, nil
}

func (s *ProcessingService) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}
