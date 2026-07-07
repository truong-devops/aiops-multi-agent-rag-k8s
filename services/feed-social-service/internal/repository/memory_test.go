package repository

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

func TestMemoryStoreUpsertFeedItemFromReadyVideoIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	input := readyVideoInput("evt_123", "vid_123", now)
	item, err := domain.NewFeedItemFromReadyVideo(input, now)
	if err != nil {
		t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
	}

	created, ok, err := store.UpsertFeedItemFromReadyVideo(context.Background(), input, item)
	if err != nil {
		t.Fatalf("UpsertFeedItemFromReadyVideo() error = %v", err)
	}
	if !ok {
		t.Fatal("created = false, want true")
	}
	again, ok, err := store.UpsertFeedItemFromReadyVideo(context.Background(), input, item)
	if err != nil {
		t.Fatalf("UpsertFeedItemFromReadyVideo(duplicate) error = %v", err)
	}
	if ok {
		t.Fatal("created duplicate = true, want false")
	}
	if again.VideoID != created.VideoID {
		t.Fatalf("duplicate video id = %s, want %s", again.VideoID, created.VideoID)
	}
}

func TestMemoryStoreListFeedItems(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	for _, input := range []domain.ReadyVideoInput{
		readyVideoInput("evt_1", "vid_1", now.Add(-time.Minute)),
		readyVideoInput("evt_2", "vid_2", now),
		readyVideoInput("evt_3", "vid_3", now.Add(-2*time.Minute)),
	} {
		item, err := domain.NewFeedItemFromReadyVideo(input, input.ReadyAt)
		if err != nil {
			t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
		}
		if _, _, err := store.UpsertFeedItemFromReadyVideo(context.Background(), input, item); err != nil {
			t.Fatalf("UpsertFeedItemFromReadyVideo() error = %v", err)
		}
	}

	items, err := store.ListFeedItems(context.Background(), ListFeedFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListFeedItems() error = %v", err)
	}
	if len(items) != 2 || items[0].Item.VideoID != "vid_2" || items[1].Item.VideoID != "vid_1" {
		t.Fatalf("items = %#v", items)
	}
	cursor := items[1].Item.ReadyAt
	next, err := store.ListFeedItems(context.Background(), ListFeedFilter{Limit: 2, BeforeReadyAt: &cursor, BeforeVideoID: items[1].Item.VideoID})
	if err != nil {
		t.Fatalf("ListFeedItems(cursor) error = %v", err)
	}
	if len(next) != 1 || next[0].Item.VideoID != "vid_3" {
		t.Fatalf("next = %#v", next)
	}
}

func TestMemoryStoreLikeIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	insertFeedItem(t, store, readyVideoInput("evt_like_repo", "vid_like_repo", now))

	counters, changed, err := store.SetVideoLike(context.Background(), SocialMutation{VideoID: "vid_like_repo", UserID: "usr_123", Now: now}, true)
	if err != nil {
		t.Fatalf("SetVideoLike() error = %v", err)
	}
	if !changed || counters.LikeCount != 1 {
		t.Fatalf("first like changed=%v counters=%#v", changed, counters)
	}
	counters, changed, err = store.SetVideoLike(context.Background(), SocialMutation{VideoID: "vid_like_repo", UserID: "usr_123", Now: now}, true)
	if err != nil {
		t.Fatalf("SetVideoLike(duplicate) error = %v", err)
	}
	if changed || counters.LikeCount != 1 {
		t.Fatalf("duplicate like changed=%v counters=%#v", changed, counters)
	}
	counters, changed, err = store.SetVideoLike(context.Background(), SocialMutation{VideoID: "vid_like_repo", UserID: "usr_123", Now: now}, false)
	if err != nil {
		t.Fatalf("SetVideoLike(unlike) error = %v", err)
	}
	if !changed || counters.LikeCount != 0 {
		t.Fatalf("unlike changed=%v counters=%#v", changed, counters)
	}
}

func TestMemoryStoreCommentLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	insertFeedItem(t, store, readyVideoInput("evt_comment_repo", "vid_comment_repo", now))
	comment, err := domain.NewComment(domain.CommentInput{VideoID: "vid_comment_repo", UserID: "usr_123", Body: "hello"}, now)
	if err != nil {
		t.Fatalf("NewComment() error = %v", err)
	}

	created, counters, err := store.CreateComment(context.Background(), comment)
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}
	if created.ID == "" || counters.CommentCount != 1 {
		t.Fatalf("created=%#v counters=%#v", created, counters)
	}
	comments, err := store.ListComments(context.Background(), ListCommentsFilter{VideoID: "vid_comment_repo", Limit: 10})
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if len(comments) != 1 || comments[0].ID != created.ID {
		t.Fatalf("comments=%#v", comments)
	}
	deleted, counters, changed, err := store.DeleteComment(context.Background(), created.ID, "usr_123", "", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	if !changed || deleted.Body != "" || counters.CommentCount != 0 {
		t.Fatalf("deleted=%#v changed=%v counters=%#v", deleted, changed, counters)
	}
}

func TestMemoryStoreFollowLifecycle(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	follow, changed, err := store.SetFollow(context.Background(), FollowMutation{
		FollowerID: "usr_viewer",
		FolloweeID: "usr_creator",
		Now:        now,
	}, true)
	if err != nil {
		t.Fatalf("SetFollow() error = %v", err)
	}
	if !changed || follow.Status != domain.FollowStatusActive {
		t.Fatalf("follow=%#v changed=%v", follow, changed)
	}
	follow, changed, err = store.SetFollow(context.Background(), FollowMutation{
		FollowerID: "usr_viewer",
		FolloweeID: "usr_creator",
		Now:        now,
	}, true)
	if err != nil {
		t.Fatalf("SetFollow(duplicate) error = %v", err)
	}
	if changed || follow.Status != domain.FollowStatusActive {
		t.Fatalf("duplicate follow=%#v changed=%v", follow, changed)
	}
	follow, changed, err = store.SetFollow(context.Background(), FollowMutation{
		FollowerID: "usr_viewer",
		FolloweeID: "usr_creator",
		Now:        now,
	}, false)
	if err != nil {
		t.Fatalf("SetFollow(unfollow) error = %v", err)
	}
	if !changed || follow.Status != domain.FollowStatusDeleted {
		t.Fatalf("unfollow=%#v changed=%v", follow, changed)
	}
}

func insertFeedItem(t *testing.T, store *MemoryStore, input domain.ReadyVideoInput) {
	t.Helper()
	item, err := domain.NewFeedItemFromReadyVideo(input, input.ReadyAt)
	if err != nil {
		t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
	}
	if _, _, err := store.UpsertFeedItemFromReadyVideo(context.Background(), input, item); err != nil {
		t.Fatalf("UpsertFeedItemFromReadyVideo() error = %v", err)
	}
}

func readyVideoInput(eventID string, videoID string, readyAt time.Time) domain.ReadyVideoInput {
	return domain.ReadyVideoInput{
		EventID:            eventID,
		VideoID:            videoID,
		OwnerID:            "usr_123",
		Title:              "Video " + videoID,
		ThumbnailObjectKey: "thumbnails/" + videoID + "/poster.jpg",
		PlaybackObjectKey:  "processed/" + videoID + "/source.mp4",
		DurationMs:         1000,
		Visibility:         "public",
		ReadyAt:            readyAt,
		ReceivedAt:         readyAt,
	}
}
