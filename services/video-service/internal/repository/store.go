package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type ListVideosFilter struct {
	OwnerID string
	Status  string
	Limit   int
}

type Store interface {
	CreateVideoWithUploadRequest(ctx context.Context, video domain.Video, upload domain.UploadRequest, history domain.StatusHistory) error
	FindVideoByID(ctx context.Context, id string) (domain.Video, error)
	ListVideos(ctx context.Context, filter ListVideosFilter) ([]domain.Video, error)
	FindUploadRequestByID(ctx context.Context, id string) (domain.UploadRequest, error)
	FindUploadIntentByIdempotencyKey(ctx context.Context, ownerID string, idempotencyKey string) (domain.Video, domain.UploadRequest, error)
	SaveUploadRequest(ctx context.Context, upload domain.UploadRequest) error
	CompleteUpload(ctx context.Context, upload domain.UploadRequest, video domain.Video, history domain.StatusHistory, outbox domain.OutboxEvent) error
	SaveVideoStatus(ctx context.Context, video domain.Video, history domain.StatusHistory) error
	ListPendingOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkOutboxFailed(ctx context.Context, id string, errMessage string) error
	Ping(ctx context.Context) error
}
