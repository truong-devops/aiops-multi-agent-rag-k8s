package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type MemoryStore struct {
	mu          sync.RWMutex
	jobs        map[string]domain.ProcessingJob
	jobsByVideo map[string]string
	inboxEvents map[string]domain.InboxEvent
	attempts    map[string]domain.ProcessingAttempt
	deadLetters map[string]domain.DeadLetter
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:        map[string]domain.ProcessingJob{},
		jobsByVideo: map[string]string{},
		inboxEvents: map[string]domain.InboxEvent{},
		attempts:    map[string]domain.ProcessingAttempt{},
		deadLetters: map[string]domain.DeadLetter{},
	}
}

func (s *MemoryStore) CreateJobFromUploadedEvent(_ context.Context, event domain.UploadedVideoEvent, job domain.ProcessingJob) (domain.ProcessingJob, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.inboxEvents[event.EventID]; ok {
		if jobID, ok := s.jobsByVideo[existing.AggregateID]; ok {
			return s.jobs[jobID], false, nil
		}
	}
	if jobID, ok := s.jobsByVideo[event.VideoID]; ok {
		return s.jobs[jobID], false, nil
	}
	processedAt := job.CreatedAt
	s.inboxEvents[event.EventID] = domain.InboxEvent{
		ID:            event.EventID,
		EventName:     "video.uploaded",
		EventVersion:  "v1",
		AggregateID:   event.VideoID,
		Status:        domain.InboxStatusProcessed,
		RequestID:     event.RequestID,
		CorrelationID: event.CorrelationID,
		ReceivedAt:    event.ReceivedAt,
		ProcessedAt:   &processedAt,
	}
	s.jobs[job.ID] = job
	s.jobsByVideo[job.VideoID] = job.ID
	return job, true, nil
}

func (s *MemoryStore) FindJobByID(_ context.Context, id string) (domain.ProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.ProcessingJob{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	return job, nil
}

func (s *MemoryStore) FindJobByVideoID(_ context.Context, videoID string) (domain.ProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobID, ok := s.jobsByVideo[videoID]
	if !ok {
		return domain.ProcessingJob{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	return s.jobs[jobID], nil
}

func (s *MemoryStore) ListJobs(_ context.Context, filter ListJobsFilter) ([]domain.ProcessingJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]domain.ProcessingJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		if filter.VideoID != "" && job.VideoID != filter.VideoID {
			continue
		}
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Priority == jobs[j].Priority {
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		}
		return jobs[i].Priority > jobs[j].Priority
	})
	if filter.Limit > 0 && len(jobs) > filter.Limit {
		jobs = jobs[:filter.Limit]
	}
	return jobs, nil
}

func (s *MemoryStore) ClaimRunnableJobs(_ context.Context, workerID string, now time.Time, leaseTTL time.Duration, limit int) ([]domain.ProcessingJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 10
	}
	candidates := make([]domain.ProcessingJob, 0)
	for _, job := range s.jobs {
		if job.Status != domain.JobStatusQueued && job.Status != domain.JobStatusRetrying {
			continue
		}
		if job.NextRunAt.After(now) {
			continue
		}
		candidates = append(candidates, job)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].NextRunAt.Before(candidates[j].NextRunAt)
		}
		return candidates[i].Priority > candidates[j].Priority
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	claimed := make([]domain.ProcessingJob, 0, len(candidates))
	for _, job := range candidates {
		lockedUntil := now.Add(leaseTTL)
		job.LockedBy = workerID
		job.LockedUntil = &lockedUntil
		job.UpdatedAt = now
		s.jobs[job.ID] = job
		claimed = append(claimed, job)
	}
	return claimed, nil
}

func (s *MemoryStore) StartAttempt(_ context.Context, jobID string, workerID string, now time.Time) (domain.ProcessingJob, domain.ProcessingAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	if job.Status != domain.JobStatusQueued && job.Status != domain.JobStatusRetrying && job.Status != domain.JobStatusRunning {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, domain.Conflict(domain.CodeInvalidJobState, "Processing job cannot start an attempt.")
	}
	job.Status = domain.JobStatusRunning
	job.AttemptCount++
	job.LockedBy = workerID
	job.UpdatedAt = now
	if job.StartedAt == nil {
		startedAt := now
		job.StartedAt = &startedAt
	}
	attempt := domain.ProcessingAttempt{
		ID:        domain.NewID("att"),
		JobID:     job.ID,
		VideoID:   job.VideoID,
		AttemptNo: job.AttemptCount,
		WorkerID:  workerID,
		Status:    domain.AttemptStatusRunning,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
		Metrics:   []byte(`{}`),
	}
	s.jobs[job.ID] = job
	s.attempts[attempt.ID] = attempt
	return job, attempt, nil
}

func (s *MemoryStore) MarkAttemptSucceeded(_ context.Context, jobID string, attemptID string, now time.Time, metrics []byte) (domain.ProcessingJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, attempt, err := s.jobAndAttempt(jobID, attemptID)
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	finishedAt := now
	attempt.Status = domain.AttemptStatusSucceeded
	attempt.FinishedAt = &finishedAt
	attempt.Metrics = metricsOrEmpty(metrics)
	attempt.UpdatedAt = now
	job.Status = domain.JobStatusSucceeded
	job.CompletedAt = &finishedAt
	job.LockedBy = ""
	job.LockedUntil = nil
	job.UpdatedAt = now
	s.jobs[job.ID] = job
	s.attempts[attempt.ID] = attempt
	return job, nil
}

func (s *MemoryStore) MarkAttemptFailed(_ context.Context, jobID string, attemptID string, now time.Time, errorCode string, errorMessage string, retryAt *time.Time, deadLetter *domain.DeadLetter) (domain.ProcessingJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, attempt, err := s.jobAndAttempt(jobID, attemptID)
	if err != nil {
		return domain.ProcessingJob{}, err
	}
	finishedAt := now
	attempt.Status = domain.AttemptStatusFailed
	attempt.FinishedAt = &finishedAt
	attempt.ErrorCode = errorCode
	attempt.StderrExcerpt = errorMessage
	attempt.UpdatedAt = now
	job.ErrorCode = errorCode
	job.ErrorMessage = errorMessage
	job.UpdatedAt = now
	if deadLetter != nil {
		job.Status = domain.JobStatusDeadLetter
		job.CompletedAt = &finishedAt
		job.LockedBy = ""
		job.LockedUntil = nil
		s.deadLetters[deadLetter.ID] = *deadLetter
	} else if retryAt != nil {
		job.Status = domain.JobStatusRetrying
		job.NextRunAt = retryAt.UTC()
		job.LockedBy = ""
		job.LockedUntil = nil
	} else {
		job.Status = domain.JobStatusFailed
		job.CompletedAt = &finishedAt
		job.LockedBy = ""
		job.LockedUntil = nil
	}
	s.jobs[job.ID] = job
	s.attempts[attempt.ID] = attempt
	return job, nil
}

func (s *MemoryStore) Stats(_ context.Context, now time.Time) (StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now = now.UTC()
	stats := StoreStats{JobStatusCounts: map[string]int64{}}
	var oldestRunnable *time.Time
	for _, job := range s.jobs {
		stats.JobStatusCounts[job.Status]++
		if job.Status != domain.JobStatusQueued && job.Status != domain.JobStatusRetrying {
			continue
		}
		if job.NextRunAt.After(now) {
			continue
		}
		if job.LockedUntil != nil && job.LockedUntil.After(now) {
			continue
		}
		stats.RunnableCount++
		nextRunAt := job.NextRunAt
		if oldestRunnable == nil || nextRunAt.Before(*oldestRunnable) {
			oldestRunnable = &nextRunAt
		}
	}
	if oldestRunnable != nil && oldestRunnable.Before(now) {
		stats.OldestRunnableAge = now.Sub(*oldestRunnable)
	}
	return stats, nil
}

func (s *MemoryStore) Ping(context.Context) error {
	return nil
}

func (s *MemoryStore) jobAndAttempt(jobID string, attemptID string) (domain.ProcessingJob, domain.ProcessingAttempt, error) {
	job, ok := s.jobs[jobID]
	if !ok {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, domain.NotFound(domain.CodeJobNotFound, "Processing job was not found.")
	}
	attempt, ok := s.attempts[attemptID]
	if !ok {
		return domain.ProcessingJob{}, domain.ProcessingAttempt{}, domain.NotFound(domain.CodeAttemptNotFound, "Processing attempt was not found.")
	}
	return job, attempt, nil
}

func metricsOrEmpty(metrics []byte) []byte {
	if len(metrics) == 0 {
		return []byte(`{}`)
	}
	return metrics
}
