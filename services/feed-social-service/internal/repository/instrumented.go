package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type DBMetrics interface {
	RecordDBOperation(operation string, outcome string, duration time.Duration)
}

type InstrumentedStore struct {
	next    Store
	metrics DBMetrics
	now     func() time.Time
}

func NewInstrumentedStore(next Store, metrics DBMetrics) Store {
	return InstrumentedStore{next: next, metrics: metrics, now: time.Now}
}

func (s InstrumentedStore) UpsertFeedItemFromReadyVideo(ctx context.Context, input domain.ReadyVideoInput, item domain.FeedItem) (domain.FeedItem, bool, error) {
	startedAt := s.now()
	out, created, err := s.next.UpsertFeedItemFromReadyVideo(ctx, input, item)
	s.record("upsert_feed_item_from_ready_video", err, startedAt)
	return out, created, err
}

func (s InstrumentedStore) FindFeedItemByVideoID(ctx context.Context, videoID string) (domain.FeedItemWithCounters, error) {
	startedAt := s.now()
	out, err := s.next.FindFeedItemByVideoID(ctx, videoID)
	s.record("find_feed_item_by_video_id", err, startedAt)
	return out, err
}

func (s InstrumentedStore) ListFeedItems(ctx context.Context, filter ListFeedFilter) ([]domain.FeedItemWithCounters, error) {
	startedAt := s.now()
	out, err := s.next.ListFeedItems(ctx, filter)
	s.record("list_feed_items", err, startedAt)
	return out, err
}

func (s InstrumentedStore) Ping(ctx context.Context) error {
	startedAt := s.now()
	err := s.next.Ping(ctx)
	s.record("ping", err, startedAt)
	return err
}

func (s InstrumentedStore) record(operation string, err error, startedAt time.Time) {
	if s.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	s.metrics.RecordDBOperation(operation, outcome, s.now().Sub(startedAt))
}
