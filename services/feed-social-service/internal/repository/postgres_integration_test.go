package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

func TestPostgresStoreFeedItemFlow(t *testing.T) {
	databaseURL := os.Getenv("FEED_SOCIAL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set FEED_SOCIAL_TEST_DATABASE_URL to run postgres integration tests")
	}

	ctx := context.Background()
	store, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}()

	applyFeedMigrations(t, store.db)
	truncateFeedTables(t, store.db)

	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	input := domain.ReadyVideoInput{
		EventID:            "evt_feed_pgtest",
		VideoID:            "vid_feed_pgtest",
		OwnerID:            "usr_feed_pgtest",
		Title:              "Postgres feed test",
		ThumbnailObjectKey: "thumbnails/vid_feed_pgtest/poster.jpg",
		PlaybackObjectKey:  "processed/vid_feed_pgtest/source.mp4",
		DurationMs:         1000,
		Visibility:         "public",
		RequestID:          "req_feed_pgtest",
		CorrelationID:      "corr_feed_pgtest",
		ReadyAt:            now,
		ReceivedAt:         now,
	}
	item, err := domain.NewFeedItemFromReadyVideo(input, now)
	if err != nil {
		t.Fatalf("NewFeedItemFromReadyVideo() error = %v", err)
	}

	created, ok, err := store.UpsertFeedItemFromReadyVideo(ctx, input, item)
	if err != nil {
		t.Fatalf("UpsertFeedItemFromReadyVideo() error = %v", err)
	}
	if !ok || created.VideoID != input.VideoID {
		t.Fatalf("created=%v item=%#v", ok, created)
	}
	again, ok, err := store.UpsertFeedItemFromReadyVideo(ctx, input, item)
	if err != nil {
		t.Fatalf("UpsertFeedItemFromReadyVideo(duplicate) error = %v", err)
	}
	if ok || again.VideoID != input.VideoID {
		t.Fatalf("duplicate created=%v item=%#v", ok, again)
	}
	items, err := store.ListFeedItems(ctx, ListFeedFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListFeedItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Item.VideoID != input.VideoID {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Counters.VideoID != input.VideoID {
		t.Fatalf("counters = %#v", items[0].Counters)
	}
}

func applyFeedMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, name := range []string{"001_feed_schema.sql"} {
		path := filepath.Join("..", "..", "migrations", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func truncateFeedTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		TRUNCATE TABLE
			video_social_counters,
			feed_items,
			inbox_events
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate feed tables: %v", err)
	}
}
