package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/event"
)

func TestPostgresStoreUploadFlowWithOutbox(t *testing.T) {
	databaseURL := os.Getenv("VIDEO_SERVICE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set VIDEO_SERVICE_TEST_DATABASE_URL to run postgres integration tests")
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

	applyVideoMigrations(t, store.db)
	truncateVideoTables(t, store.db)

	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	video := domain.Video{
		ID:                "vid_pgtest",
		OwnerID:           "usr_pgtest",
		Title:             "Postgres upload flow",
		Description:       "integration",
		Status:            domain.VideoStatusDraft,
		Visibility:        domain.VisibilityPublic,
		RawObjectKey:      "raw/vid_pgtest/source.mp4",
		ContentType:       "video/mp4",
		SizeBytes:         1024,
		CreatedAt:         now,
		UpdatedAt:         now,
		LastRequestID:     "req_create",
		LastCorrelationID: "corr_pgtest",
	}
	upload := domain.UploadRequest{
		ID:            "upl_pgtest",
		VideoID:       video.ID,
		OwnerID:       video.OwnerID,
		Bucket:        "raw-videos",
		ObjectKey:     video.RawObjectKey,
		Status:        domain.UploadStatusCreated,
		ContentType:   "video/mp4",
		SizeBytes:     1024,
		ExpiresAt:     now.Add(time.Hour),
		CreatedAt:     now,
		UpdatedAt:     now,
		RequestID:     "req_create",
		CorrelationID: "corr_pgtest",
	}
	history := domain.StatusHistory{
		ID:            "vsh_pgtest_created",
		VideoID:       video.ID,
		NewStatus:     domain.VideoStatusDraft,
		Reason:        "upload_request_created",
		RequestID:     "req_create",
		CorrelationID: "corr_pgtest",
		CreatedAt:     now,
	}
	if err := store.CreateVideoWithUploadRequest(ctx, video, upload, history); err != nil {
		t.Fatalf("CreateVideoWithUploadRequest() error = %v", err)
	}

	foundVideo, err := store.FindVideoByID(ctx, video.ID)
	if err != nil {
		t.Fatalf("FindVideoByID() error = %v", err)
	}
	if foundVideo.Status != domain.VideoStatusDraft || foundVideo.OwnerID != video.OwnerID {
		t.Fatalf("found video = %#v", foundVideo)
	}

	completedAt := now.Add(5 * time.Minute)
	upload.Status = domain.UploadStatusUploaded
	upload.SizeBytes = 2048
	upload.CompletedAt = &completedAt
	upload.UpdatedAt = completedAt
	upload.RequestID = "req_confirm"
	video.Status = domain.VideoStatusUploaded
	video.SizeBytes = 2048
	video.UpdatedAt = completedAt
	video.LastRequestID = "req_confirm"
	video.LastCorrelationID = "corr_pgtest"
	confirmedHistory := domain.StatusHistory{
		ID:             "vsh_pgtest_uploaded",
		VideoID:        video.ID,
		PreviousStatus: domain.VideoStatusDraft,
		NewStatus:      domain.VideoStatusUploaded,
		Reason:         "upload_confirmed",
		RequestID:      "req_confirm",
		CorrelationID:  "corr_pgtest",
		CreatedAt:      completedAt,
	}
	outbox, err := event.NewVideoUploadedOutbox(video, "test", completedAt)
	if err != nil {
		t.Fatalf("NewVideoUploadedOutbox() error = %v", err)
	}
	outbox.ID = "evt_pgtest_uploaded"

	if err := store.CompleteUpload(ctx, upload, video, confirmedHistory, outbox); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	uploaded, err := store.FindVideoByID(ctx, video.ID)
	if err != nil {
		t.Fatalf("FindVideoByID(uploaded) error = %v", err)
	}
	if uploaded.Status != domain.VideoStatusUploaded || uploaded.SizeBytes != 2048 {
		t.Fatalf("uploaded video = %#v", uploaded)
	}

	listed, err := store.ListVideos(ctx, ListVideosFilter{OwnerID: video.OwnerID, Status: domain.VideoStatusUploaded, Limit: 10})
	if err != nil {
		t.Fatalf("ListVideos() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != video.ID {
		t.Fatalf("listed videos = %#v", listed)
	}

	var eventName string
	var eventVersion string
	var eventStatus string
	var payloadRaw []byte
	err = store.db.QueryRowContext(ctx, `
		SELECT event_name, event_version, status, payload
		FROM outbox_events
		WHERE id = $1
	`, outbox.ID).Scan(&eventName, &eventVersion, &eventStatus, &payloadRaw)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}
	if eventName != event.VideoUploadedName || eventVersion != event.VideoUploadedVersion || eventStatus != domain.OutboxStatusPending {
		t.Fatalf("outbox envelope = %s/%s/%s", eventName, eventVersion, eventStatus)
	}
	var payload event.VideoUploadedPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if payload.VideoID != video.ID || payload.OwnerID != video.OwnerID || payload.SizeBytes != 2048 {
		t.Fatalf("outbox payload = %#v", payload)
	}
}

func applyVideoMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, name := range []string{"001_video_schema.sql", "002_outbox_envelope.sql", "003_idempotency_outbox_attempts.sql"} {
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

func truncateVideoTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		TRUNCATE TABLE
			outbox_events,
			video_status_history,
			video_assets,
			upload_requests,
			videos
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate video tables: %v", err)
	}
}
