package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type ListFeedFilter struct {
	Limit         int
	BeforeReadyAt *time.Time
	BeforeVideoID string
}

type Store interface {
	UpsertFeedItemFromReadyVideo(ctx context.Context, input domain.ReadyVideoInput, item domain.FeedItem) (domain.FeedItem, bool, error)
	FindFeedItemByVideoID(ctx context.Context, videoID string) (domain.FeedItemWithCounters, error)
	ListFeedItems(ctx context.Context, filter ListFeedFilter) ([]domain.FeedItemWithCounters, error)
	Ping(ctx context.Context) error
}
