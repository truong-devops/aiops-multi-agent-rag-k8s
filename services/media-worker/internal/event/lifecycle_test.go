package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

func TestBuildReadyLifecycleEvent(t *testing.T) {
	envelope := BuildReady(lifecycleJob(), lifecycleAttempt(), ProcessingOutput{
		ProcessedObjectKey: "processed/vid_123/source.mp4",
		ThumbnailObjectKey: "thumbnails/vid_123/poster.jpg",
		DurationMs:         12340,
		Width:              1280,
		Height:             720,
		SizeBytes:          4096,
	}, LifecycleOptions{
		Producer:    "media-worker",
		Environment: "test",
		OccurredAt:  time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC),
	})

	if envelope.EventType != VideoReadyFullName || envelope.EventName != VideoReadyName || envelope.EventVersion != "v1" {
		t.Fatalf("envelope identity = %#v", envelope)
	}
	raw, err := MarshalLifecycleEvent(envelope)
	if err != nil {
		t.Fatalf("MarshalLifecycleEvent() error = %v", err)
	}
	var decoded struct {
		EventType string `json:"event_type"`
		Payload   struct {
			VideoID            string `json:"video_id"`
			JobID              string `json:"job_id"`
			AttemptID          string `json:"attempt_id"`
			Status             string `json:"status"`
			ProcessedObjectKey string `json:"processed_object_key"`
			ThumbnailObjectKey string `json:"thumbnail_object_key"`
			DurationMs         int64  `json:"duration_ms"`
			Width              int    `json:"width"`
			Height             int    `json:"height"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode lifecycle event: %v", err)
	}
	if decoded.EventType != VideoReadyFullName {
		t.Fatalf("event_type = %q", decoded.EventType)
	}
	if decoded.Payload.VideoID != "vid_123" ||
		decoded.Payload.JobID != "job_123" ||
		decoded.Payload.AttemptID != "att_123" ||
		decoded.Payload.Status != domain.JobStatusSucceeded ||
		decoded.Payload.ProcessedObjectKey != "processed/vid_123/source.mp4" ||
		decoded.Payload.ThumbnailObjectKey != "thumbnails/vid_123/poster.jpg" ||
		decoded.Payload.DurationMs != 12340 ||
		decoded.Payload.Width != 1280 ||
		decoded.Payload.Height != 720 {
		t.Fatalf("payload = %#v", decoded.Payload)
	}
}

func TestBuildProcessingFailedLifecycleEvent(t *testing.T) {
	envelope := BuildProcessingFailed(lifecycleJob(), lifecycleAttempt(), domain.CodeFFmpegFailed, LifecycleOptions{
		Environment: "test",
		OccurredAt:  time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC),
	})

	raw, err := MarshalLifecycleEvent(envelope)
	if err != nil {
		t.Fatalf("MarshalLifecycleEvent() error = %v", err)
	}
	var decoded struct {
		EventType string `json:"event_type"`
		Payload   struct {
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode lifecycle event: %v", err)
	}
	if decoded.EventType != VideoProcessingFailedFullName ||
		decoded.Payload.Status != domain.JobStatusFailed ||
		decoded.Payload.ErrorCode != domain.CodeFFmpegFailed {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func lifecycleJob() domain.ProcessingJob {
	return domain.ProcessingJob{
		ID:            "job_123",
		VideoID:       "vid_123",
		OwnerID:       "usr_123",
		RequestID:     "req_123",
		CorrelationID: "corr_123",
	}
}

func lifecycleAttempt() domain.ProcessingAttempt {
	return domain.ProcessingAttempt{
		ID:       "att_123",
		JobID:    "job_123",
		VideoID:  "vid_123",
		WorkerID: "worker_123",
	}
}
