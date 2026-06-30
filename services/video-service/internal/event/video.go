package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

const (
	ProducerVideoService  = "video-service"
	VideoUploadedName     = "video.uploaded"
	VideoUploadedVersion  = "v1"
	VideoUploadedFullName = "video.uploaded.v1"
)

type VideoUploadedPayload struct {
	VideoID      string `json:"video_id"`
	OwnerID      string `json:"owner_id"`
	RawObjectKey string `json:"raw_object_key"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
}

func NewVideoUploadedOutbox(video domain.Video, environment string, occurredAt time.Time) (domain.OutboxEvent, error) {
	payload := VideoUploadedPayload{
		VideoID:      video.ID,
		OwnerID:      video.OwnerID,
		RawObjectKey: video.RawObjectKey,
		ContentType:  video.ContentType,
		SizeBytes:    video.SizeBytes,
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("marshal %s payload: %w", VideoUploadedFullName, err)
	}
	environment = strings.TrimSpace(environment)
	if environment == "" {
		environment = "local"
	}
	return domain.OutboxEvent{
		ID:            domain.NewID("evt"),
		EventName:     VideoUploadedName,
		EventVersion:  VideoUploadedVersion,
		AggregateID:   video.ID,
		Producer:      ProducerVideoService,
		Environment:   environment,
		Payload:       rawPayload,
		Status:        domain.OutboxStatusPending,
		RequestID:     video.LastRequestID,
		CorrelationID: video.LastCorrelationID,
		OccurredAt:    occurredAt.UTC(),
		CreatedAt:     occurredAt.UTC(),
	}, nil
}
