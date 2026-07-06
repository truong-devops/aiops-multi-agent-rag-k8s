package service

import (
	"context"
	"testing"
	"time"

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

func upsertReady(t *testing.T, svc *FeedService, input domain.ReadyVideoInput) {
	t.Helper()
	if _, _, err := svc.UpsertReadyVideo(context.Background(), input); err != nil {
		t.Fatalf("UpsertReadyVideo() error = %v", err)
	}
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
