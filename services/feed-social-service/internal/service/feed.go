package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/cache"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
)

type FeedService struct {
	store        repository.Store
	metrics      MetricsRecorder
	logger       *slog.Logger
	now          func() time.Time
	defaultLimit int
	maxLimit     int
	cache        cache.Store
	cacheTTL     time.Duration
}

type Options struct {
	Metrics      MetricsRecorder
	Logger       *slog.Logger
	Now          func() time.Time
	DefaultLimit int
	MaxLimit     int
	Cache        cache.Store
	CacheTTL     time.Duration
}

type MetricsRecorder interface {
	RecordFeedOperation(operation string, outcome string)
	RecordFeedResult(operation string, outcome string, count int)
	RecordCacheOperation(operation string, outcome string, duration time.Duration)
}

type FeedQuery struct {
	Limit  int
	Cursor string
}

type Actor struct {
	UserID string
	Role   string
}

type CommentQuery struct {
	VideoID string
	Limit   int
	Cursor  string
}

type CreateCommentInput struct {
	VideoID       string
	Actor         Actor
	Body          string
	RequestID     string
	CorrelationID string
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

type CommentPage struct {
	Comments   []domain.Comment
	Limit      int
	NextCursor string
	HasMore    bool
}

type commentCursor struct {
	CreatedAt string `json:"created_at"`
	CommentID string `json:"comment_id"`
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
	cacheStore := options.Cache
	if cacheStore == nil {
		cacheStore = cache.NewNoopStore()
	}
	cacheTTL := options.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = time.Minute
	}
	return &FeedService{
		store:        store,
		metrics:      options.Metrics,
		logger:       logger,
		now:          now,
		defaultLimit: defaultLimit,
		maxLimit:     maxLimit,
		cache:        cacheStore,
		cacheTTL:     cacheTTL,
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
	cacheKey := feedCacheKey(limit, query.Cursor)
	if page, ok := s.getCachedFeed(ctx, cacheKey); ok {
		return FeedPage{Items: page.Items, Limit: page.Limit, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
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
	page := FeedPage{
		Items:      items,
		Limit:      limit,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
	s.setCachedFeed(ctx, cacheKey, cache.FeedPage(page))
	return page, nil
}

func (s *FeedService) GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return domain.VideoSocialCounters{}, domain.ValidationError("video_id is required.")
	}
	if counters, ok := s.getCachedCounters(ctx, videoID); ok {
		return counters, nil
	}
	counters, err := s.store.GetSocialCounters(ctx, videoID)
	if err != nil {
		s.record("get_social_counters", "error")
		return domain.VideoSocialCounters{}, err
	}
	s.record("get_social_counters", "success")
	s.setCachedCounters(ctx, videoID, counters)
	return counters, nil
}

func (s *FeedService) LikeVideo(ctx context.Context, videoID string, actor Actor, requestID string, correlationID string) (domain.VideoSocialCounters, bool, error) {
	return s.setLike(ctx, videoID, actor, requestID, correlationID, true)
}

func (s *FeedService) UnlikeVideo(ctx context.Context, videoID string, actor Actor, requestID string, correlationID string) (domain.VideoSocialCounters, bool, error) {
	return s.setLike(ctx, videoID, actor, requestID, correlationID, false)
}

func (s *FeedService) CreateComment(ctx context.Context, input CreateCommentInput) (domain.Comment, domain.VideoSocialCounters, error) {
	actor, err := requireActor(input.Actor)
	if err != nil {
		s.record("create_comment", "unauthorized")
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	comment, err := domain.NewComment(domain.CommentInput{
		VideoID:       input.VideoID,
		UserID:        actor.UserID,
		Body:          input.Body,
		RequestID:     input.RequestID,
		CorrelationID: input.CorrelationID,
	}, s.now().UTC())
	if err != nil {
		s.record("create_comment", "invalid")
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	created, counters, err := s.store.CreateComment(ctx, comment)
	if err != nil {
		s.record("create_comment", "error")
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	s.invalidateSocialCaches(ctx, created.VideoID)
	s.record("create_comment", "created")
	s.logger.Info(
		"comment created",
		"service", "feed-social-service",
		"video_id", created.VideoID,
		"comment_id", created.ID,
		"user_id", created.UserID,
		"request_id", created.RequestID,
		"correlation_id", created.CorrelationID,
	)
	return created, counters, nil
}

func (s *FeedService) ListComments(ctx context.Context, query CommentQuery) (CommentPage, error) {
	videoID := strings.TrimSpace(query.VideoID)
	if videoID == "" {
		return CommentPage{}, domain.ValidationError("video_id is required.")
	}
	limit, err := s.normalizeLimit(query.Limit)
	if err != nil {
		s.record("list_comments", "invalid")
		return CommentPage{}, err
	}
	filter := repository.ListCommentsFilter{VideoID: videoID, Limit: limit + 1}
	if strings.TrimSpace(query.Cursor) != "" {
		createdAt, commentID, err := decodeCommentCursor(query.Cursor)
		if err != nil {
			s.record("list_comments", "invalid_cursor")
			return CommentPage{}, err
		}
		filter.BeforeCreatedAt = &createdAt
		filter.BeforeCommentID = commentID
	}
	comments, err := s.store.ListComments(ctx, filter)
	if err != nil {
		s.record("list_comments", "error")
		return CommentPage{}, err
	}
	hasMore := len(comments) > limit
	if hasMore {
		comments = comments[:limit]
	}
	nextCursor := ""
	if hasMore && len(comments) > 0 {
		nextCursor = encodeCommentCursor(comments[len(comments)-1])
	}
	s.record("list_comments", "success")
	s.recordResult("list_comments", "success", len(comments))
	return CommentPage{
		Comments:   comments,
		Limit:      limit,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *FeedService) DeleteComment(ctx context.Context, commentID string, actor Actor) (domain.Comment, domain.VideoSocialCounters, bool, error) {
	actor, err := requireActor(actor)
	if err != nil {
		s.record("delete_comment", "unauthorized")
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		s.record("delete_comment", "invalid")
		return domain.Comment{}, domain.VideoSocialCounters{}, false, domain.ValidationError("comment_id is required.")
	}
	comment, counters, changed, err := s.store.DeleteComment(ctx, commentID, actor.UserID, actor.Role, s.now().UTC())
	if err != nil {
		s.record("delete_comment", "error")
		return domain.Comment{}, domain.VideoSocialCounters{}, false, err
	}
	if changed {
		s.record("delete_comment", "deleted")
		s.invalidateSocialCaches(ctx, comment.VideoID)
	} else {
		s.record("delete_comment", "noop")
	}
	return comment, counters, changed, nil
}

func (s *FeedService) FollowUser(ctx context.Context, followeeID string, actor Actor, requestID string, correlationID string) (domain.Follow, bool, error) {
	return s.setFollow(ctx, followeeID, actor, requestID, correlationID, true)
}

func (s *FeedService) UnfollowUser(ctx context.Context, followeeID string, actor Actor, requestID string, correlationID string) (domain.Follow, bool, error) {
	return s.setFollow(ctx, followeeID, actor, requestID, correlationID, false)
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
		s.invalidateFeedCache(ctx)
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

func (s *FeedService) setLike(ctx context.Context, videoID string, actor Actor, requestID string, correlationID string, liked bool) (domain.VideoSocialCounters, bool, error) {
	actor, err := requireActor(actor)
	if err != nil {
		s.record("set_like", "unauthorized")
		return domain.VideoSocialCounters{}, false, err
	}
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		s.record("set_like", "invalid")
		return domain.VideoSocialCounters{}, false, domain.ValidationError("video_id is required.")
	}
	counters, changed, err := s.store.SetVideoLike(ctx, repository.SocialMutation{
		VideoID:       videoID,
		UserID:        actor.UserID,
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(correlationID),
		Now:           s.now().UTC(),
	}, liked)
	if err != nil {
		s.record("set_like", "error")
		return domain.VideoSocialCounters{}, false, err
	}
	if changed {
		s.invalidateSocialCaches(ctx, videoID)
		s.setCachedCounters(ctx, videoID, counters)
	}
	outcome := "noop"
	if changed && liked {
		outcome = "liked"
	} else if changed {
		outcome = "unliked"
	}
	s.record("set_like", outcome)
	return counters, changed, nil
}

func (s *FeedService) getCachedFeed(ctx context.Context, key string) (cache.FeedPage, bool) {
	startedAt := s.now()
	page, ok, err := s.cache.GetFeed(ctx, key)
	if err != nil {
		s.recordCache("feed_get", "error", startedAt)
		s.logger.Warn("feed cache read failed", "service", "feed-social-service", "error", err)
		return cache.FeedPage{}, false
	}
	if ok {
		s.recordCache("feed_get", "hit", startedAt)
		return page, true
	}
	s.recordCache("feed_get", "miss", startedAt)
	return cache.FeedPage{}, false
}

func (s *FeedService) setCachedFeed(ctx context.Context, key string, page cache.FeedPage) {
	startedAt := s.now()
	if err := s.cache.SetFeed(ctx, key, page, s.cacheTTL); err != nil {
		s.recordCache("feed_set", "error", startedAt)
		s.logger.Warn("feed cache write failed", "service", "feed-social-service", "error", err)
		return
	}
	s.recordCache("feed_set", "success", startedAt)
}

func (s *FeedService) getCachedCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, bool) {
	startedAt := s.now()
	counters, ok, err := s.cache.GetCounters(ctx, videoID)
	if err != nil {
		s.recordCache("counters_get", "error", startedAt)
		s.logger.Warn("counters cache read failed", "service", "feed-social-service", "video_id", videoID, "error", err)
		return domain.VideoSocialCounters{}, false
	}
	if ok {
		s.recordCache("counters_get", "hit", startedAt)
		return counters, true
	}
	s.recordCache("counters_get", "miss", startedAt)
	return domain.VideoSocialCounters{}, false
}

func (s *FeedService) setCachedCounters(ctx context.Context, videoID string, counters domain.VideoSocialCounters) {
	startedAt := s.now()
	if err := s.cache.SetCounters(ctx, videoID, counters, s.cacheTTL); err != nil {
		s.recordCache("counters_set", "error", startedAt)
		s.logger.Warn("counters cache write failed", "service", "feed-social-service", "video_id", videoID, "error", err)
		return
	}
	s.recordCache("counters_set", "success", startedAt)
}

func (s *FeedService) invalidateSocialCaches(ctx context.Context, videoID string) {
	s.invalidateFeedCache(ctx)
	startedAt := s.now()
	if err := s.cache.InvalidateCounters(ctx, videoID); err != nil {
		s.recordCache("counters_invalidate", "error", startedAt)
		s.logger.Warn("counters cache invalidation failed", "service", "feed-social-service", "video_id", videoID, "error", err)
		return
	}
	s.recordCache("counters_invalidate", "success", startedAt)
}

func (s *FeedService) invalidateFeedCache(ctx context.Context) {
	startedAt := s.now()
	if err := s.cache.InvalidateFeed(ctx); err != nil {
		s.recordCache("feed_invalidate", "error", startedAt)
		s.logger.Warn("feed cache invalidation failed", "service", "feed-social-service", "error", err)
		return
	}
	s.recordCache("feed_invalidate", "success", startedAt)
}

func (s *FeedService) recordCache(operation string, outcome string, startedAt time.Time) {
	if s.metrics != nil {
		s.metrics.RecordCacheOperation(operation, outcome, s.now().Sub(startedAt))
	}
}

func feedCacheKey(limit int, cursor string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(cursor)))
	return strings.TrimSpace(hex.EncodeToString(sum[:])) + ":" + strings.TrimSpace(jsonNumber(limit))
}

func jsonNumber(value int) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func (s *FeedService) setFollow(ctx context.Context, followeeID string, actor Actor, requestID string, correlationID string, following bool) (domain.Follow, bool, error) {
	actor, err := requireActor(actor)
	if err != nil {
		s.record("set_follow", "unauthorized")
		return domain.Follow{}, false, err
	}
	followeeID = strings.TrimSpace(followeeID)
	if followeeID == "" {
		s.record("set_follow", "invalid")
		return domain.Follow{}, false, domain.ValidationError("followee user_id is required.")
	}
	if followeeID == actor.UserID {
		s.record("set_follow", "self_follow")
		return domain.Follow{}, false, domain.ValidationError("user cannot follow themselves.")
	}
	follow, changed, err := s.store.SetFollow(ctx, repository.FollowMutation{
		FollowerID:    actor.UserID,
		FolloweeID:    followeeID,
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(correlationID),
		Now:           s.now().UTC(),
	}, following)
	if err != nil {
		s.record("set_follow", "error")
		return domain.Follow{}, false, err
	}
	outcome := "noop"
	if changed && following {
		outcome = "followed"
	} else if changed {
		outcome = "unfollowed"
	}
	s.record("set_follow", outcome)
	return follow, changed, nil
}

func requireActor(actor Actor) (Actor, error) {
	actor.UserID = strings.TrimSpace(actor.UserID)
	actor.Role = strings.TrimSpace(actor.Role)
	if actor.UserID == "" {
		return Actor{}, domain.NewError(http.StatusUnauthorized, domain.CodeUnauthorized, "Trusted user context is required.")
	}
	return actor, nil
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

func encodeCommentCursor(comment domain.Comment) string {
	raw, err := json.Marshal(commentCursor{
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339Nano),
		CommentID: comment.ID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCommentCursor(value string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	var cursor commentCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cursor.CreatedAt))
	if err != nil {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	commentID := strings.TrimSpace(cursor.CommentID)
	if commentID == "" {
		return time.Time{}, "", domain.NewError(http.StatusBadRequest, domain.CodeValidationError, "cursor is invalid.")
	}
	return createdAt.UTC(), commentID, nil
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
