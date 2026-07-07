package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/cache"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
)

func TestListFeedCapsLimitAndReturnsActiveItems(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{DefaultLimit: 2, MaxLimit: 2})
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	upsertReady(t, svc, readyInput("evt_1", "vid_1", "public", now))
	upsertReady(t, svc, readyInput("evt_2", "vid_2", "private", now.Add(time.Minute)))
	upsertReady(t, svc, readyInput("evt_3", "vid_3", "public", now.Add(2*time.Minute)))

	page, err := svc.ListFeed(context.Background(), FeedQuery{Limit: 99})
	if err != nil {
		t.Fatalf("ListFeed() error = %v", err)
	}
	if page.Limit != 2 || page.HasMore || len(page.Items) != 2 {
		t.Fatalf("page = %#v", page)
	}
	if page.Items[0].Item.VideoID != "vid_3" || page.Items[1].Item.VideoID != "vid_1" {
		t.Fatalf("items = %#v", page.Items)
	}
}

func TestListFeedRejectsInvalidCursor(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{DefaultLimit: 2, MaxLimit: 2})

	if _, err := svc.ListFeed(context.Background(), FeedQuery{Cursor: "not-base64"}); err == nil {
		t.Fatal("ListFeed() error = nil, want invalid cursor error")
	}
}

func TestLikeVideoRequiresActorAndIsIdempotent(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{DefaultLimit: 2, MaxLimit: 2})
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	upsertReady(t, svc, readyInput("evt_like_service", "vid_like_service", "public", now))

	if _, _, err := svc.LikeVideo(context.Background(), "vid_like_service", Actor{}, "req_1", "corr_1"); err == nil {
		t.Fatal("LikeVideo() error = nil, want actor error")
	}
	counters, changed, err := svc.LikeVideo(context.Background(), "vid_like_service", Actor{UserID: "usr_123"}, "req_1", "corr_1")
	if err != nil {
		t.Fatalf("LikeVideo() error = %v", err)
	}
	if !changed || counters.LikeCount != 1 {
		t.Fatalf("first like changed=%v counters=%#v", changed, counters)
	}
	counters, changed, err = svc.LikeVideo(context.Background(), "vid_like_service", Actor{UserID: "usr_123"}, "req_2", "corr_2")
	if err != nil {
		t.Fatalf("LikeVideo(duplicate) error = %v", err)
	}
	if changed || counters.LikeCount != 1 {
		t.Fatalf("duplicate like changed=%v counters=%#v", changed, counters)
	}
}

func TestCommentServiceLifecycle(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{DefaultLimit: 1, MaxLimit: 1})
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	upsertReady(t, svc, readyInput("evt_comment_service", "vid_comment_service", "public", now))
	comment, counters, err := svc.CreateComment(context.Background(), CreateCommentInput{
		VideoID: "vid_comment_service",
		Actor:   Actor{UserID: "usr_123"},
		Body:    "hello",
	})
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if comment.ID == "" || counters.CommentCount != 1 {
		t.Fatalf("comment=%#v counters=%#v", comment, counters)
	}
	page, err := svc.ListComments(context.Background(), CommentQuery{VideoID: "vid_comment_service"})
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(page.Comments) != 1 || page.Comments[0].ID != comment.ID {
		t.Fatalf("page=%#v", page)
	}
	deleted, counters, changed, err := svc.DeleteComment(context.Background(), comment.ID, Actor{UserID: "usr_123"})
	if err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	if !changed || deleted.Body != "" || counters.CommentCount != 0 {
		t.Fatalf("deleted=%#v changed=%v counters=%#v", deleted, changed, counters)
	}
}

func TestFollowServiceLifecycle(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{})
	if _, _, err := svc.FollowUser(context.Background(), "usr_creator", Actor{}, "req_1", "corr_1"); err == nil {
		t.Fatal("FollowUser() error = nil, want actor error")
	}
	if _, _, err := svc.FollowUser(context.Background(), "usr_viewer", Actor{UserID: "usr_viewer"}, "req_1", "corr_1"); err == nil {
		t.Fatal("FollowUser(self) error = nil, want validation error")
	}
	follow, changed, err := svc.FollowUser(context.Background(), "usr_creator", Actor{UserID: "usr_viewer"}, "req_1", "corr_1")
	if err != nil {
		t.Fatalf("FollowUser() error = %v", err)
	}
	if !changed || follow.Status != domain.FollowStatusActive {
		t.Fatalf("follow=%#v changed=%v", follow, changed)
	}
	follow, changed, err = svc.UnfollowUser(context.Background(), "usr_creator", Actor{UserID: "usr_viewer"}, "req_2", "corr_2")
	if err != nil {
		t.Fatalf("UnfollowUser() error = %v", err)
	}
	if !changed || follow.Status != domain.FollowStatusDeleted {
		t.Fatalf("unfollow=%#v changed=%v", follow, changed)
	}
}

func TestListFeedFallsBackWhenCacheFails(t *testing.T) {
	svc := NewFeedService(repository.NewMemoryStore(), Options{Cache: failingCache{}, DefaultLimit: 2, MaxLimit: 2})
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	upsertReady(t, svc, readyInput("evt_cache_service", "vid_cache_service", "public", now))

	page, err := svc.ListFeed(context.Background(), FeedQuery{})
	if err != nil {
		t.Fatalf("ListFeed() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Item.VideoID != "vid_cache_service" {
		t.Fatalf("page=%#v", page)
	}
}

func upsertReady(t *testing.T, svc *FeedService, input domain.ReadyVideoInput) {
	t.Helper()
	if _, _, err := svc.UpsertReadyVideo(context.Background(), input); err != nil {
		t.Fatalf("UpsertReadyVideo() error = %v", err)
	}
}

type failingCache struct{}

func (failingCache) GetFeed(context.Context, string) (cache.FeedPage, bool, error) {
	return cache.FeedPage{}, false, errors.New("cache down")
}

func (failingCache) SetFeed(context.Context, string, cache.FeedPage, time.Duration) error {
	return errors.New("cache down")
}

func (failingCache) InvalidateFeed(context.Context) error {
	return errors.New("cache down")
}

func (failingCache) GetCounters(context.Context, string) (domain.VideoSocialCounters, bool, error) {
	return domain.VideoSocialCounters{}, false, errors.New("cache down")
}

func (failingCache) SetCounters(context.Context, string, domain.VideoSocialCounters, time.Duration) error {
	return errors.New("cache down")
}

func (failingCache) InvalidateCounters(context.Context, string) error {
	return errors.New("cache down")
}

func (failingCache) Close() error {
	return nil
}

func readyInput(eventID string, videoID string, visibility string, readyAt time.Time) domain.ReadyVideoInput {
	return domain.ReadyVideoInput{
		EventID:            eventID,
		VideoID:            videoID,
		OwnerID:            "usr_123",
		Title:              "Video " + videoID,
		ThumbnailObjectKey: "thumbnails/" + videoID + "/poster.jpg",
		PlaybackObjectKey:  "processed/" + videoID + "/source.mp4",
		Visibility:         visibility,
		ReadyAt:            readyAt,
		ReceivedAt:         readyAt,
	}
}
