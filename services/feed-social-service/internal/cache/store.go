package cache

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type FeedPage struct {
	Items      []domain.FeedItemWithCounters `json:"items"`
	Limit      int                           `json:"limit"`
	NextCursor string                        `json:"next_cursor"`
	HasMore    bool                          `json:"has_more"`
}

type Store interface {
	GetFeed(ctx context.Context, key string) (FeedPage, bool, error)
	SetFeed(ctx context.Context, key string, page FeedPage, ttl time.Duration) error
	InvalidateFeed(ctx context.Context) error
	GetCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, bool, error)
	SetCounters(ctx context.Context, videoID string, counters domain.VideoSocialCounters, ttl time.Duration) error
	InvalidateCounters(ctx context.Context, videoID string) error
	Close() error
}

type NoopStore struct{}

func NewNoopStore() NoopStore {
	return NoopStore{}
}

func (NoopStore) GetFeed(context.Context, string) (FeedPage, bool, error) {
	return FeedPage{}, false, nil
}

func (NoopStore) SetFeed(context.Context, string, FeedPage, time.Duration) error {
	return nil
}

func (NoopStore) InvalidateFeed(context.Context) error {
	return nil
}

func (NoopStore) GetCounters(context.Context, string) (domain.VideoSocialCounters, bool, error) {
	return domain.VideoSocialCounters{}, false, nil
}

func (NoopStore) SetCounters(context.Context, string, domain.VideoSocialCounters, time.Duration) error {
	return nil
}

func (NoopStore) InvalidateCounters(context.Context, string) error {
	return nil
}

func (NoopStore) Close() error {
	return nil
}
