package repository

import (
	"context"

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
	SaveUploadRequest(ctx context.Context, upload domain.UploadRequest) error
	CompleteUpload(ctx context.Context, upload domain.UploadRequest, video domain.Video, history domain.StatusHistory, outbox domain.OutboxEvent) error
	SaveVideoStatus(ctx context.Context, video domain.Video, history domain.StatusHistory) error
	Ping(ctx context.Context) error
}
