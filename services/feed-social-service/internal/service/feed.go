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
	store        repository.Store
	metrics      MetricsRecorder
	logger       *slog.Logger
	now          func() time.Time
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

func (s *FeedService) GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return domain.VideoSocialCounters{}, domain.ValidationError("video_id is required.")
	}
	counters, err := s.store.GetSocialCounters(ctx, videoID)
	if err != nil {
		s.record("get_social_counters", "error")
		return domain.VideoSocialCounters{}, err
	}
	s.record("get_social_counters", "success")
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
	} else {
		s.record("delete_comment", "noop")
	}
	return comment, counters, changed, nil
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
	outcome := "noop"
	if changed && liked {
		outcome = "liked"
	} else if changed {
		outcome = "unliked"
	}
	s.record("set_like", outcome)
	return counters, changed, nil
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
