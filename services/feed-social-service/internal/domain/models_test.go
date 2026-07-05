package domain

import (
	"testing"
	"time"
)

func TestNewFeedItemFromReadyVideo(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	item, err := NewFeedItemFromReadyVideo(ReadyVideoInput{
		VideoID:            "vid_123",
		OwnerID:            "usr_123",
		Title:              "Hello",
		ThumbnailObjectKey: "thumbnails/vid_123/poster.jpg",
		PlaybackObjectKey:  "processed/vid_123/source.mp4",
		DurationMs:         12340,
	}, now)
	if err != nil {
		t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
	}
	if item.ID != "feed_vid_123" || item.Status != FeedItemStatusActive || item.Visibility != "public" {
		t.Fatalf("item = %#v", item)
	}
	if item.ReadyAt != now {
		t.Fatalf("ReadyAt = %s, want %s", item.ReadyAt, now)
	}
}

func TestNewFeedItemFromReadyVideoHidesNonPublic(t *testing.T) {
	item, err := NewFeedItemFromReadyVideo(ReadyVideoInput{
		VideoID:    "vid_123",
		OwnerID:    "usr_123",
		Visibility: "private",
	}, time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
	}
	if item.Status != FeedItemStatusHidden {
		t.Fatalf("status = %q, want hidden", item.Status)
	}
}

func TestFeedItemTransitions(t *testing.T) {
	if !CanTransitionFeedItem(FeedItemStatusActive, FeedItemStatusHidden) {
		t.Fatal("active -> hidden should be allowed")
	}
	if CanTransitionFeedItem(FeedItemStatusDeleted, FeedItemStatusActive) {
		t.Fatal("deleted -> active should not be allowed")
	}
}
