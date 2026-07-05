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
