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

type SocialMutation struct {
	VideoID       string
	UserID        string
	RequestID     string
	CorrelationID string
	Now           time.Time
}

type FollowMutation struct {
	FollowerID    string
	FolloweeID    string
	RequestID     string
	CorrelationID string
	Now           time.Time
}

type ListCommentsFilter struct {
	VideoID         string
	Limit           int
	BeforeCreatedAt *time.Time
	BeforeCommentID string
}

type Store interface {
	UpsertFeedItemFromReadyVideo(ctx context.Context, input domain.ReadyVideoInput, item domain.FeedItem) (domain.FeedItem, bool, error)
	FindFeedItemByVideoID(ctx context.Context, videoID string) (domain.FeedItemWithCounters, error)
	ListFeedItems(ctx context.Context, filter ListFeedFilter) ([]domain.FeedItemWithCounters, error)
	GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error)
	SetVideoLike(ctx context.Context, mutation SocialMutation, liked bool) (domain.VideoSocialCounters, bool, error)
	CreateComment(ctx context.Context, comment domain.Comment) (domain.Comment, domain.VideoSocialCounters, error)
	ListComments(ctx context.Context, filter ListCommentsFilter) ([]domain.Comment, error)
	DeleteComment(ctx context.Context, commentID string, actorID string, actorRole string, now time.Time) (domain.Comment, domain.VideoSocialCounters, bool, error)
	SetFollow(ctx context.Context, mutation FollowMutation, following bool) (domain.Follow, bool, error)
	Ping(ctx context.Context) error
}
