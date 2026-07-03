package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

const (
	VideoUploadedName     = "video.uploaded"
	VideoUploadedVersion  = "v1"
	VideoUploadedFullName = "video.uploaded.v1"
)

type Envelope struct {
	EventID       string          `json:"event_id"`
	EventName     string          `json:"event_name"`
	EventVersion  string          `json:"event_version"`
	EventType     string          `json:"event_type"`
	AggregateID   string          `json:"aggregate_id"`
	Producer      string          `json:"producer"`
	Environment   string          `json:"environment"`
	CorrelationID string          `json:"correlation_id"`
	RequestID     string          `json:"request_id"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

type VideoUploadedPayload struct {
	VideoID      string `json:"video_id"`
	OwnerID      string `json:"owner_id"`
	RawObjectKey string `json:"raw_object_key"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
}

func ParseUploadedEvent(raw []byte, receivedAt time.Time) (domain.UploadedVideoEvent, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return domain.UploadedVideoEvent{}, fmt.Errorf("decode event envelope: %w", err)
	}
	if strings.TrimSpace(envelope.EventID) == "" {
		return domain.UploadedVideoEvent{}, domain.ValidationError("event_id is required.")
	}
	if !isVideoUploaded(envelope) {
		return domain.UploadedVideoEvent{}, domain.ValidationError("event type must be video.uploaded.v1.")
	}
	var payload VideoUploadedPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return domain.UploadedVideoEvent{}, fmt.Errorf("decode video.uploaded payload: %w", err)
	}
	occurredAt := receivedAt.UTC()
	if strings.TrimSpace(envelope.OccurredAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, envelope.OccurredAt)
		if err != nil {
			return domain.UploadedVideoEvent{}, domain.ValidationError("occurred_at must be RFC3339 timestamp.")
		}
		occurredAt = parsed.UTC()
	}
	event := domain.UploadedVideoEvent{
		EventID:       strings.TrimSpace(envelope.EventID),
		VideoID:       strings.TrimSpace(payload.VideoID),
		OwnerID:       strings.TrimSpace(payload.OwnerID),
		RawObjectKey:  strings.TrimSpace(payload.RawObjectKey),
		ContentType:   strings.TrimSpace(payload.ContentType),
		SizeBytes:     payload.SizeBytes,
		RequestID:     strings.TrimSpace(envelope.RequestID),
		CorrelationID: strings.TrimSpace(envelope.CorrelationID),
		OccurredAt:    occurredAt,
		ReceivedAt:    receivedAt.UTC(),
	}
	if event.VideoID == "" {
		return domain.UploadedVideoEvent{}, domain.ValidationError("payload.video_id is required.")
	}
	if event.RawObjectKey == "" {
		return domain.UploadedVideoEvent{}, domain.ValidationError("payload.raw_object_key is required.")
	}
	return event, nil
}

func isVideoUploaded(envelope Envelope) bool {
	eventType := strings.TrimSpace(envelope.EventType)
	if eventType == "" && envelope.EventName != "" && envelope.EventVersion != "" {
		eventType = envelope.EventName + "." + envelope.EventVersion
	}
	if eventType == VideoUploadedFullName {
		return true
	}
	return strings.TrimSpace(envelope.EventName) == VideoUploadedName &&
		strings.TrimSpace(envelope.EventVersion) == VideoUploadedVersion
}
