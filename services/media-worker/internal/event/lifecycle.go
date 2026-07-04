package event

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

const (
	VideoProcessingStartedName     = "video.processing_started"
	VideoProcessingStartedFullName = "video.processing_started.v1"
	VideoReadyName                 = "video.ready"
	VideoReadyFullName             = "video.ready.v1"
	VideoProcessingFailedName      = "video.processing_failed"
	VideoProcessingFailedFullName  = "video.processing_failed.v1"
)

type LifecycleEnvelope struct {
	EventID       string `json:"event_id"`
	EventName     string `json:"event_name"`
	EventVersion  string `json:"event_version"`
	EventType     string `json:"event_type"`
	AggregateID   string `json:"aggregate_id"`
	Producer      string `json:"producer"`
	Environment   string `json:"environment"`
	CorrelationID string `json:"correlation_id"`
	RequestID     string `json:"request_id"`
	OccurredAt    string `json:"occurred_at"`
	Payload       any    `json:"payload"`
}

type LifecyclePayload struct {
	VideoID            string `json:"video_id"`
	OwnerID            string `json:"owner_id,omitempty"`
	JobID              string `json:"job_id"`
	AttemptID          string `json:"attempt_id"`
	WorkerID           string `json:"worker_id,omitempty"`
	Status             string `json:"status"`
	ProcessedObjectKey string `json:"processed_object_key,omitempty"`
	ThumbnailObjectKey string `json:"thumbnail_object_key,omitempty"`
	DurationMs         int64  `json:"duration_ms,omitempty"`
	Width              int    `json:"width,omitempty"`
	Height             int    `json:"height,omitempty"`
	SizeBytes          int64  `json:"size_bytes,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
}

type ProcessingOutput struct {
	ProcessedObjectKey string
	ThumbnailObjectKey string
	DurationMs         int64
	Width              int
	Height             int
	SizeBytes          int64
}

type LifecycleOptions struct {
	Producer    string
	Environment string
	OccurredAt  time.Time
}

func BuildProcessingStarted(job domain.ProcessingJob, attempt domain.ProcessingAttempt, options LifecycleOptions) LifecycleEnvelope {
	return lifecycleEnvelope(VideoProcessingStartedName, VideoProcessingStartedFullName, job, attempt, domain.JobStatusRunning, options, LifecyclePayload{})
}

func BuildReady(job domain.ProcessingJob, attempt domain.ProcessingAttempt, output ProcessingOutput, options LifecycleOptions) LifecycleEnvelope {
	payload := LifecyclePayload{
		ProcessedObjectKey: output.ProcessedObjectKey,
		ThumbnailObjectKey: output.ThumbnailObjectKey,
		DurationMs:         output.DurationMs,
		Width:              output.Width,
		Height:             output.Height,
		SizeBytes:          output.SizeBytes,
	}
	return lifecycleEnvelope(VideoReadyName, VideoReadyFullName, job, attempt, domain.JobStatusSucceeded, options, payload)
}

func BuildProcessingFailed(job domain.ProcessingJob, attempt domain.ProcessingAttempt, errorCode string, options LifecycleOptions) LifecycleEnvelope {
	payload := LifecyclePayload{ErrorCode: strings.TrimSpace(errorCode)}
	return lifecycleEnvelope(VideoProcessingFailedName, VideoProcessingFailedFullName, job, attempt, domain.JobStatusFailed, options, payload)
}

func MarshalLifecycleEvent(envelope LifecycleEnvelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func lifecycleEnvelope(eventName string, eventType string, job domain.ProcessingJob, attempt domain.ProcessingAttempt, status string, options LifecycleOptions, payload LifecyclePayload) LifecycleEnvelope {
	occurredAt := options.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload.VideoID = job.VideoID
	payload.OwnerID = job.OwnerID
	payload.JobID = job.ID
	payload.AttemptID = attempt.ID
	payload.WorkerID = attempt.WorkerID
	payload.Status = status
	return LifecycleEnvelope{
		EventID:       domain.NewID("evt"),
		EventName:     eventName,
		EventVersion:  "v1",
		EventType:     eventType,
		AggregateID:   job.VideoID,
		Producer:      defaultString(options.Producer, "media-worker"),
		Environment:   defaultString(options.Environment, "local"),
		CorrelationID: job.CorrelationID,
		RequestID:     job.RequestID,
		OccurredAt:    occurredAt.Format(time.RFC3339Nano),
		Payload:       payload,
	}
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
