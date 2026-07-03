package domain

import "time"

const (
	JobStatusQueued     = "queued"
	JobStatusRunning    = "running"
	JobStatusRetrying   = "retrying"
	JobStatusSucceeded  = "succeeded"
	JobStatusFailed     = "failed"
	JobStatusDeadLetter = "dead_letter"
	JobStatusCancelled  = "cancelled"

	AttemptStatusRunning   = "running"
	AttemptStatusSucceeded = "succeeded"
	AttemptStatusFailed    = "failed"

	InboxStatusProcessed = "processed"
	InboxStatusDuplicate = "duplicate"
)

type UploadedVideoEvent struct {
	EventID       string
	VideoID       string
	OwnerID       string
	RawObjectKey  string
	ContentType   string
	SizeBytes     int64
	RequestID     string
	CorrelationID string
	OccurredAt    time.Time
	ReceivedAt    time.Time
}

type ProcessingJob struct {
	ID             string
	VideoID        string
	OwnerID        string
	InputBucket    string
	InputObjectKey string
	ContentType    string
	SizeBytes      int64
	Status         string
	Priority       int
	AttemptCount   int
	MaxAttempts    int
	LockedBy       string
	LockedUntil    *time.Time
	NextRunAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	ErrorCode      string
	ErrorMessage   string
	RequestID      string
	CorrelationID  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProcessingAttempt struct {
	ID                string
	JobID             string
	VideoID           string
	AttemptNo         int
	WorkerID          string
	Status            string
	FFmpegCommandHash string
	StartedAt         time.Time
	FinishedAt        *time.Time
	ExitCode          *int
	ErrorCode         string
	StderrExcerpt     string
	Metrics           []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DeadLetter struct {
	ID            string
	JobID         string
	VideoID       string
	ReasonCode    string
	Payload       []byte
	RequestID     string
	CorrelationID string
	CreatedAt     time.Time
}

type InboxEvent struct {
	ID            string
	EventName     string
	EventVersion  string
	AggregateID   string
	Status        string
	RequestID     string
	CorrelationID string
	ReceivedAt    time.Time
	ProcessedAt   *time.Time
}

func ValidJobStatus(value string) bool {
	switch value {
	case JobStatusQueued, JobStatusRunning, JobStatusRetrying, JobStatusSucceeded, JobStatusFailed, JobStatusDeadLetter, JobStatusCancelled:
		return true
	default:
		return false
	}
}

func ValidAttemptStatus(value string) bool {
	switch value {
	case AttemptStatusRunning, AttemptStatusSucceeded, AttemptStatusFailed:
		return true
	default:
		return false
	}
}

func CanTransitionJob(from string, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case JobStatusQueued:
		return to == JobStatusRunning || to == JobStatusCancelled
	case JobStatusRunning:
		return to == JobStatusSucceeded || to == JobStatusRetrying || to == JobStatusFailed || to == JobStatusDeadLetter
	case JobStatusRetrying:
		return to == JobStatusRunning || to == JobStatusCancelled
	case JobStatusFailed:
		return to == JobStatusDeadLetter
	default:
		return false
	}
}

func NewProcessingJobFromUploadedEvent(event UploadedVideoEvent, rawBucket string, maxAttempts int, now time.Time) (ProcessingJob, error) {
	if event.EventID == "" {
		return ProcessingJob{}, ValidationError("event_id is required.")
	}
	if event.VideoID == "" {
		return ProcessingJob{}, ValidationError("video_id is required.")
	}
	if event.RawObjectKey == "" {
		return ProcessingJob{}, ValidationError("raw_object_key is required.")
	}
	if rawBucket == "" {
		return ProcessingJob{}, ValidationError("input bucket is required.")
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return ProcessingJob{
		ID:             NewID("job"),
		VideoID:        event.VideoID,
		OwnerID:        event.OwnerID,
		InputBucket:    rawBucket,
		InputObjectKey: event.RawObjectKey,
		ContentType:    event.ContentType,
		SizeBytes:      event.SizeBytes,
		Status:         JobStatusQueued,
		MaxAttempts:    maxAttempts,
		NextRunAt:      now,
		RequestID:      event.RequestID,
		CorrelationID:  event.CorrelationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}
