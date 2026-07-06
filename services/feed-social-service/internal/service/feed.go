package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
)

type FeedService struct {
	store   repository.Store
	metrics MetricsRecorder
	logger  *slog.Logger
	now     func() time.Time
	defaultLimit int
	maxLimit     int
}

type Options struct {
	Metrics      MetricsRecorder
	Logger       *slog.Logger
	Now          func() time.Time
	DefaultLimit int
	MaxLimit     int
}

type MetricsRecorder interface {
	RecordFeedOperation(operation string, outcome string)
	RecordFeedResult(operation string, outcome string, count int)
}

type FeedQuery struct {
	Limit  int
	Cursor string
}

type FeedPage struct {
	Items      []domain.FeedItemWithCounters
	Limit      int
	NextCursor string
	HasMore    bool
}

type feedCursor struct {
	ReadyAt string `json:"ready_at"`
	VideoID string `json:"video_id"`
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
	defaultLimit := options.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	maxLimit := options.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 50
	}
	if defaultLimit > maxLimit {
		defaultLimit = maxLimit
	}
	return &FeedService{
		store:        store,
		metrics:      options.Metrics,
		logger:       logger,
		now:          now,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
	}
}

func (s *FeedService) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *FeedService) ListFeed(ctx context.Context, query FeedQuery) (FeedPage, error) {
	limit, err := s.normalizeLimit(query.Limit)
	if err != nil {
		s.record("list_feed", "invalid")
		return FeedPage{}, err
	}
	filter := repository.ListFeedFilter{Limit: limit + 1}
	if strings.TrimSpace(query.Cursor) != "" {
		readyAt, videoID, err := decodeCursor(query.Cursor)
		if err != nil {
			s.record("list_feed", "invalid_cursor")
			return FeedPage{}, err
		}
		filter.BeforeReadyAt = &readyAt
		filter.BeforeVideoID = videoID
	}
	items, err := s.store.ListFeedItems(ctx, filter)
	if err != nil {
		s.record("list_feed", "error")
		return FeedPage{}, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		nextCursor = encodeCursor(items[len(items)-1].Item)
	}
	s.record("list_feed", "success")
	s.recordResult("list_feed", "success", len(items))
	return FeedPage{
		Items:      items,
		Limit:      limit,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
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

func (s *FeedService) recordResult(operation string, outcome string, count int) {
	if s.metrics != nil {
		s.metrics.RecordFeedResult(operation, outcome, count)
	}
}

func (s *FeedService) normalizeLimit(limit int) (int, error) {
	if limit < 0 {
		return 0, domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "limit must be positive.")
	}
	if limit == 0 {
		return s.defaultLimit, nil
	}
	if limit > s.maxLimit {
		return s.maxLimit, nil
	}
	return limit, nil
}

func encodeCursor(item domain.FeedItem) string {
	raw, err := json.Marshal(feedCursor{
		ReadyAt: item.ReadyAt.UTC().Format(time.RFC3339Nano),
		VideoID: item.VideoID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(value string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	var cursor feedCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	readyAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cursor.ReadyAt))
	if err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	videoID := strings.TrimSpace(cursor.VideoID)
	if videoID == "" {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	return readyAt.UTC(), videoID, nil
}
