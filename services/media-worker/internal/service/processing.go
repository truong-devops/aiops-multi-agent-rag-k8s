package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/client"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/processor"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/repository"
)

type ProcessingService struct {
	store        repository.Store
	rawBucket    string
	maxAttempts  int
	workerID     string
	leaseTTL     time.Duration
	batchSize    int
	processor    processor.Processor
	statusClient client.VideoStatusClient
	metrics      MetricsRecorder
	logger       *slog.Logger
	now          func() time.Time
}

type Options struct {
	RawBucket    string
	MaxAttempts  int
	WorkerID     string
	LeaseTTL     time.Duration
	BatchSize    int
	Processor    processor.Processor
	StatusClient client.VideoStatusClient
	Metrics      MetricsRecorder
	Logger       *slog.Logger
	Now          func() time.Time
}

type MetricsRecorder interface {
	RecordJobOperation(operation string, outcome string)
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
	leaseTTL := options.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = 2 * time.Minute
	}
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &ProcessingService{
		store:        store,
		rawBucket:    options.RawBucket,
		maxAttempts:  maxAttempts,
		workerID:     options.WorkerID,
		leaseTTL:     leaseTTL,
		batchSize:    batchSize,
		processor:    options.Processor,
		statusClient: options.StatusClient,
		metrics:      options.Metrics,
		logger:       logger,
		now:          now,
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

func (s *ProcessingService) RunOnce(ctx context.Context) error {
	now := s.now().UTC()
	jobs, err := s.store.ClaimRunnableJobs(ctx, s.workerID, now, s.leaseTTL, s.batchSize)
	if err != nil {
		s.record("runner", "claim_error")
		return err
	}
	for _, job := range jobs {
		if err := s.ProcessJob(ctx, job); err != nil {
			s.logger.Error(
				"processing job failed",
				"job_id", job.ID,
				"video_id", job.VideoID,
				"error", err,
			)
		}
	}
	if len(jobs) > 0 {
		s.record("runner", "claimed")
	}
	return nil
}

func (s *ProcessingService) Run(ctx context.Context, pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	s.logger.Info("processing runner started", "poll_interval", pollInterval.String(), "worker_id", s.workerID)
	_ = s.RunOnce(ctx)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("processing runner stopped", "worker_id", s.workerID)
			return
		case <-ticker.C:
			_ = s.RunOnce(ctx)
		}
	}
}

func (s *ProcessingService) ProcessJob(ctx context.Context, claimed domain.ProcessingJob) error {
	now := s.now().UTC()
	job, attempt, err := s.store.StartAttempt(ctx, claimed.ID, s.workerID, now)
	if err != nil {
		s.record("attempt", "start_error")
		return err
	}
	if err := s.updateVideoStatus(ctx, job, "processing", "worker_started", ""); err != nil {
		decision := DecideRetry(job, domain.ProcessingError{
			Code:      domain.CodeVideoServiceUnavailable,
			Message:   err.Error(),
			Retryable: true,
		}, s.now().UTC())
		_, markErr := s.markFailed(ctx, job, attempt, decision)
		if markErr != nil {
			return markErr
		}
		return err
	}
	if s.processor == nil {
		return domain.ProcessingError{Code: domain.CodeUnknownProcessingError, Message: "processor is not configured", Retryable: true}
	}
	result, err := s.processor.Process(ctx, job, attempt)
	if err != nil {
		decision := DecideRetry(job, err, s.now().UTC())
		updated, markErr := s.markFailed(ctx, job, attempt, decision)
		if markErr != nil {
			return markErr
		}
		if decision.DeadLetter {
			_ = s.updateVideoStatus(ctx, updated, "failed", "worker_failed", decision.ErrorCode)
		}
		return err
	}
	if err := s.updateVideoStatus(ctx, job, "ready", "worker_completed", ""); err != nil {
		decision := DecideRetry(job, domain.ProcessingError{
			Code:      domain.CodeVideoServiceUnavailable,
			Message:   err.Error(),
			Retryable: true,
		}, s.now().UTC())
		_, markErr := s.markFailed(ctx, job, attempt, decision)
		if markErr != nil {
			return markErr
		}
		return err
	}
	_, err = s.store.MarkAttemptSucceeded(ctx, job.ID, attempt.ID, s.now().UTC(), result.Metrics)
	if err != nil {
		s.record("attempt", "success_mark_error")
		return err
	}
	s.record("attempt", "succeeded")
	s.logger.Info(
		"processing job succeeded",
		"job_id", job.ID,
		"attempt_id", attempt.ID,
		"video_id", job.VideoID,
		"processed_object_key", result.ProcessedObjectKey,
		"thumbnail_object_key", result.ThumbnailObjectKey,
		"request_id", job.RequestID,
		"correlation_id", job.CorrelationID,
	)
	return nil
}

func (s *ProcessingService) markFailed(ctx context.Context, job domain.ProcessingJob, attempt domain.ProcessingAttempt, decision RetryDecision) (domain.ProcessingJob, error) {
	var deadLetter *domain.DeadLetter
	if decision.DeadLetter {
		payload, _ := json.Marshal(map[string]any{
			"job_id":           job.ID,
			"video_id":         job.VideoID,
			"attempt_count":    job.AttemptCount,
			"error_code":       decision.ErrorCode,
			"retryable":        decision.Retryable,
			"request_id":       job.RequestID,
			"correlation_id":   job.CorrelationID,
			"input_object_key": job.InputObjectKey,
		})
		deadLetter = &domain.DeadLetter{
			ID:            domain.NewID("dlq"),
			JobID:         job.ID,
			VideoID:       job.VideoID,
			ReasonCode:    decision.ErrorCode,
			Payload:       payload,
			RequestID:     job.RequestID,
			CorrelationID: job.CorrelationID,
			CreatedAt:     s.now().UTC(),
		}
	}
	updated, err := s.store.MarkAttemptFailed(ctx, job.ID, attempt.ID, s.now().UTC(), decision.ErrorCode, decision.ErrorMessage, decision.RetryAt, deadLetter)
	if err != nil {
		s.record("attempt", "failure_mark_error")
		return domain.ProcessingJob{}, err
	}
	if decision.DeadLetter {
		s.record("attempt", "dead_letter")
	} else {
		s.record("attempt", "retry_scheduled")
	}
	return updated, nil
}

func (s *ProcessingService) updateVideoStatus(ctx context.Context, job domain.ProcessingJob, status string, reason string, errorCode string) error {
	if s.statusClient == nil {
		return nil
	}
	err := s.statusClient.UpdateStatus(ctx, client.UpdateVideoStatusInput{
		VideoID:       job.VideoID,
		Status:        status,
		Reason:        reason,
		ErrorCode:     errorCode,
		RequestID:     job.RequestID,
		CorrelationID: job.CorrelationID,
	})
	if err != nil {
		s.record("video_status_update", "error")
		return err
	}
	s.record("video_status_update", status)
	return nil
}

func (s *ProcessingService) record(operation string, outcome string) {
	if s.metrics != nil {
		s.metrics.RecordJobOperation(operation, outcome)
	}
}
