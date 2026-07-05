package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
)

type FeedService struct {
	store   repository.Store
	metrics MetricsRecorder
	logger  *slog.Logger
	now     func() time.Time
}

type Options struct {
	Metrics MetricsRecorder
	Logger  *slog.Logger
	Now     func() time.Time
}

type MetricsRecorder interface {
	RecordFeedOperation(operation string, outcome string)
}

func NewFeedService(store repository.Store, options Options) *FeedService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &FeedService{
		store:   store,
		metrics: options.Metrics,
		logger:  logger,
		now:     now,
	}
}

func (s *FeedService) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *FeedService) UpsertReadyVideo(ctx context.Context, input domain.ReadyVideoInput) (domain.FeedItem, bool, error) {
	now := s.now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	item, err := domain.NewFeedItemFromReadyVideo(input, now)
	if err != nil {
		return domain.FeedItem{}, false, err
	}
	createdItem, created, err := s.store.UpsertFeedItemFromReadyVideo(ctx, input, item)
	if err != nil {
		s.record("upsert_ready_video", "error")
		return domain.FeedItem{}, false, err
	}
	if created {
		s.record("upsert_ready_video", "created")
		s.logger.Info(
			"feed item created",
			"service", "feed-social-service",
			"video_id", createdItem.VideoID,
			"owner_id", createdItem.OwnerID,
			"request_id", input.RequestID,
			"correlation_id", input.CorrelationID,
		)
	} else {
		s.record("upsert_ready_video", "duplicate")
	}
	return createdItem, created, nil
}

func (s *FeedService) record(operation string, outcome string) {
	if s.metrics != nil {
		s.metrics.RecordFeedOperation(operation, outcome)
	}
}
