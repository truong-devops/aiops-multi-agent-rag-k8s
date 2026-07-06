package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

const (
	VideoReadyName     = "video.ready"
	VideoReadyVersion  = "v1"
	VideoReadyFullName = "video.ready.v1"
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

type ReadyVideoPayload struct {
	VideoID            string `json:"video_id"`
	OwnerID            string `json:"owner_id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	ProcessedObjectKey string `json:"processed_object_key"`
	ThumbnailObjectKey string `json:"thumbnail_object_key"`
	DurationMs         int64  `json:"duration_ms"`
	Visibility         string `json:"visibility"`
	ReadyAt            string `json:"ready_at"`
}

func ParseReadyEvent(raw []byte, receivedAt time.Time) (domain.ReadyVideoInput, time.Time, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return domain.ReadyVideoInput{}, time.Time{}, fmt.Errorf("decode event envelope: %w", err)
	}
	if strings.TrimSpace(envelope.EventID) == "" {
		return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("event_id is required.")
	}
	if !isVideoReady(envelope) {
		return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("event type must be video.ready.v1.")
	}
	var payload ReadyVideoPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return domain.ReadyVideoInput{}, time.Time{}, fmt.Errorf("decode video.ready payload: %w", err)
	}
	occurredAt := receivedAt.UTC()
	if strings.TrimSpace(envelope.OccurredAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, envelope.OccurredAt)
		if err != nil {
			return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("occurred_at must be RFC3339 timestamp.")
		}
		occurredAt = parsed.UTC()
	}
	readyAt := occurredAt
	if strings.TrimSpace(payload.ReadyAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, payload.ReadyAt)
		if err != nil {
			return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("payload.ready_at must be RFC3339 timestamp.")
		}
		readyAt = parsed.UTC()
	}
	input := domain.ReadyVideoInput{
		EventID:            strings.TrimSpace(envelope.EventID),
		VideoID:            strings.TrimSpace(payload.VideoID),
		OwnerID:            strings.TrimSpace(payload.OwnerID),
		Title:              strings.TrimSpace(payload.Title),
		Description:        strings.TrimSpace(payload.Description),
		ThumbnailObjectKey: strings.TrimSpace(payload.ThumbnailObjectKey),
		PlaybackObjectKey:  strings.TrimSpace(payload.ProcessedObjectKey),
		DurationMs:         payload.DurationMs,
		Visibility:         strings.TrimSpace(payload.Visibility),
		RequestID:          strings.TrimSpace(envelope.RequestID),
		CorrelationID:      strings.TrimSpace(envelope.CorrelationID),
		ReadyAt:            readyAt,
		ReceivedAt:         receivedAt.UTC(),
	}
	if input.VideoID == "" {
		return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("payload.video_id is required.")
	}
	if input.OwnerID == "" {
		return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("payload.owner_id is required.")
	}
	if input.PlaybackObjectKey == "" {
		return domain.ReadyVideoInput{}, time.Time{}, domain.ValidationError("payload.processed_object_key is required.")
	}
	return input, occurredAt, nil
}

func isVideoReady(envelope Envelope) bool {
	eventType := strings.TrimSpace(envelope.EventType)
	if eventType == "" && envelope.EventName != "" && envelope.EventVersion != "" {
		eventType = envelope.EventName + "." + envelope.EventVersion
	}
	if eventType == VideoReadyFullName {
		return true
	}
	return strings.TrimSpace(envelope.EventName) == VideoReadyName &&
		strings.TrimSpace(envelope.EventVersion) == VideoReadyVersion
}
