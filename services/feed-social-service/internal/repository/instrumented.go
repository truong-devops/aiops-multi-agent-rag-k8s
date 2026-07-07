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

func (s InstrumentedStore) GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error) {
	startedAt := s.now()
	out, err := s.next.GetSocialCounters(ctx, videoID)
	s.record("get_social_counters", err, startedAt)
	return out, err
}

func (s InstrumentedStore) SetVideoLike(ctx context.Context, mutation SocialMutation, liked bool) (domain.VideoSocialCounters, bool, error) {
	startedAt := s.now()
	out, changed, err := s.next.SetVideoLike(ctx, mutation, liked)
	s.record("set_video_like", err, startedAt)
	return out, changed, err
}

func (s InstrumentedStore) CreateComment(ctx context.Context, comment domain.Comment) (domain.Comment, domain.VideoSocialCounters, error) {
	startedAt := s.now()
	out, counters, err := s.next.CreateComment(ctx, comment)
	s.record("create_comment", err, startedAt)
	return out, counters, err
}

func (s InstrumentedStore) ListComments(ctx context.Context, filter ListCommentsFilter) ([]domain.Comment, error) {
	startedAt := s.now()
	out, err := s.next.ListComments(ctx, filter)
	s.record("list_comments", err, startedAt)
	return out, err
}

func (s InstrumentedStore) DeleteComment(ctx context.Context, commentID string, actorID string, actorRole string, now time.Time) (domain.Comment, domain.VideoSocialCounters, bool, error) {
	startedAt := s.now()
	out, counters, changed, err := s.next.DeleteComment(ctx, commentID, actorID, actorRole, now)
	s.record("delete_comment", err, startedAt)
	return out, counters, changed, err
}

func (s InstrumentedStore) SetFollow(ctx context.Context, mutation FollowMutation, following bool) (domain.Follow, bool, error) {
	startedAt := s.now()
	out, changed, err := s.next.SetFollow(ctx, mutation, following)
	s.record("set_follow", err, startedAt)
	return out, changed, err
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
