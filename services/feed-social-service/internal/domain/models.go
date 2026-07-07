package domain

import (
	"strings"
	"time"
)

const (
	FeedItemStatusActive  = "active"
	FeedItemStatusHidden  = "hidden"
	FeedItemStatusDeleted = "deleted"

	LikeStatusActive  = "active"
	LikeStatusDeleted = "deleted"

	CommentStatusVisible = "visible"
	CommentStatusHidden  = "hidden"
	CommentStatusDeleted = "deleted"
	CommentStatusBlocked = "blocked"

	FollowStatusActive  = "active"
	FollowStatusDeleted = "deleted"
	FollowStatusBlocked = "blocked"

	InboxStatusProcessed = "processed"
	InboxStatusDuplicate = "duplicate"
)

type FeedItem struct {
	ID                 string
	VideoID            string
	OwnerID            string
	Title              string
	Description        string
	ThumbnailObjectKey string
	PlaybackObjectKey  string
	DurationMs         int64
	Visibility         string
	Status             string
	ReadyAt            time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type VideoSocialCounters struct {
	VideoID      string
	LikeCount    int64
	CommentCount int64
	ShareCount   int64
	UpdatedAt    time.Time
}

type FeedItemWithCounters struct {
	Item     FeedItem
	Counters VideoSocialCounters
}

type Like struct {
	ID            string
	VideoID       string
	UserID        string
	Status        string
	RequestID     string
	CorrelationID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Comment struct {
	ID            string
	VideoID       string
	UserID        string
	Body          string
	Status        string
	RequestID     string
	CorrelationID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Follow struct {
	ID            string
	FollowerID    string
	FolloweeID    string
	Status        string
	RequestID     string
	CorrelationID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type InboxEvent struct {
	ID            string
	EventName     string
	EventVersion  string
	AggregateID   string
	Status        string
	RequestID     string
	CorrelationID string
	ReceivedAt    time.Time
	ProcessedAt   *time.Time
}

type ReadyVideoInput struct {
	EventID            string
	VideoID            string
	OwnerID            string
	Title              string
	Description        string
	ThumbnailObjectKey string
	PlaybackObjectKey  string
	DurationMs         int64
	Visibility         string
	RequestID          string
	CorrelationID      string
	ReadyAt            time.Time
	ReceivedAt         time.Time
}

type CommentInput struct {
	VideoID       string
	UserID        string
	Body          string
	RequestID     string
	CorrelationID string
}

func NewFeedItemFromReadyVideo(input ReadyVideoInput, now time.Time) (FeedItem, error) {
	if strings.TrimSpace(input.VideoID) == "" {
		return FeedItem{}, ValidationError("video_id is required.")
	}
	if strings.TrimSpace(input.OwnerID) == "" {
		return FeedItem{}, ValidationError("owner_id is required.")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	readyAt := input.ReadyAt.UTC()
	if readyAt.IsZero() {
		readyAt = now
	}
	visibility := strings.TrimSpace(input.Visibility)
	if visibility == "" {
		visibility = "public"
	}
	status := FeedItemStatusActive
	if visibility != "public" {
		status = FeedItemStatusHidden
	}
	return FeedItem{
		ID:                 "feed_" + strings.TrimSpace(input.VideoID),
		VideoID:            strings.TrimSpace(input.VideoID),
		OwnerID:            strings.TrimSpace(input.OwnerID),
		Title:              strings.TrimSpace(input.Title),
		Description:        strings.TrimSpace(input.Description),
		ThumbnailObjectKey: strings.TrimSpace(input.ThumbnailObjectKey),
		PlaybackObjectKey:  strings.TrimSpace(input.PlaybackObjectKey),
		DurationMs:         input.DurationMs,
		Visibility:         visibility,
		Status:             status,
		ReadyAt:            readyAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func NewComment(input CommentInput, now time.Time) (Comment, error) {
	videoID := strings.TrimSpace(input.VideoID)
	if videoID == "" {
		return Comment{}, ValidationError("video_id is required.")
	}
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return Comment{}, ValidationError("user_id is required.")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		return Comment{}, ValidationError("comment body is required.")
	}
	if len([]rune(body)) > 2000 {
		return Comment{}, ValidationError("comment body must be at most 2000 characters.")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Comment{
		ID:            NewID("comment"),
		VideoID:       videoID,
		UserID:        userID,
		Body:          body,
		Status:        CommentStatusVisible,
		RequestID:     strings.TrimSpace(input.RequestID),
		CorrelationID: strings.TrimSpace(input.CorrelationID),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (c Comment) PublicBody() string {
	if c.Status != CommentStatusVisible {
		return ""
	}
	return c.Body
}

func ValidFeedItemStatus(value string) bool {
	switch value {
	case FeedItemStatusActive, FeedItemStatusHidden, FeedItemStatusDeleted:
		return true
	default:
		return false
	}
}

func ValidLikeStatus(value string) bool {
	switch value {
	case LikeStatusActive, LikeStatusDeleted:
		return true
	default:
		return false
	}
}

func ValidCommentStatus(value string) bool {
	switch value {
	case CommentStatusVisible, CommentStatusHidden, CommentStatusDeleted, CommentStatusBlocked:
		return true
	default:
		return false
	}
}

func ValidFollowStatus(value string) bool {
	switch value {
	case FollowStatusActive, FollowStatusDeleted, FollowStatusBlocked:
		return true
	default:
		return false
	}
}

func CanTransitionFeedItem(from string, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case FeedItemStatusActive:
		return to == FeedItemStatusHidden || to == FeedItemStatusDeleted
	case FeedItemStatusHidden:
		return to == FeedItemStatusActive || to == FeedItemStatusDeleted
	default:
		return false
	}
}
