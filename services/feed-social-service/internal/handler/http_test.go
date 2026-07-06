package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/service"
)

func TestReadyz(t *testing.T) {
	app := newTestApp(nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ready"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestReadyzReportsStoreFailure(t *testing.T) {
	app := newTestApp(errors.New("db down"))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMetrics(t *testing.T) {
	app := newTestApp(nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "feed_social_http_requests_total") {
		t.Fatalf("metrics body = %s", rec.Body.String())
	}
}

func TestListFeedEmpty(t *testing.T) {
	app, _ := newFeedTestApp(t, testOptions{})
	req := httptest.NewRequest(http.MethodGet, "/v1/feed", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body feedEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 0 || body.Page.Limit != 2 || body.Page.HasMore {
		t.Fatalf("body = %#v", body)
	}
}

func TestListFeedWithCursor(t *testing.T) {
	app, svc := newFeedTestApp(t, testOptions{})
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	insertReadyVideo(t, svc, "evt_1", "vid_1", now.Add(-time.Minute))
	insertReadyVideo(t, svc, "evt_2", "vid_2", now)

	first := httptest.NewRecorder()
	app.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/v1/feed?limit=1", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}
	var firstBody feedEnvelope
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if len(firstBody.Data) != 1 || firstBody.Data[0].VideoID != "vid_2" || !firstBody.Page.HasMore || firstBody.Page.NextCursor == "" {
		t.Fatalf("first body = %#v", firstBody)
	}

	second := httptest.NewRecorder()
	app.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/v1/feed?limit=1&cursor="+firstBody.Page.NextCursor, nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	var secondBody feedEnvelope
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(secondBody.Data) != 1 || secondBody.Data[0].VideoID != "vid_1" || secondBody.Page.HasMore {
		t.Fatalf("second body = %#v", secondBody)
	}
}

func TestListFeedCapsLimit(t *testing.T) {
	app, svc := newFeedTestApp(t, testOptions{})
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	insertReadyVideo(t, svc, "evt_1", "vid_1", now)
	insertReadyVideo(t, svc, "evt_2", "vid_2", now.Add(time.Minute))
	insertReadyVideo(t, svc, "evt_3", "vid_3", now.Add(2*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/v1/feed?limit=99", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	var body feedEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 || body.Page.Limit != 2 || !body.Page.HasMore {
		t.Fatalf("body = %#v", body)
	}
}

func TestInternalIngestionRequiresToken(t *testing.T) {
	app, _ := newFeedTestApp(t, testOptions{internalToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/feed-items", strings.NewReader(`{"video_id":"vid_1","owner_id":"usr_1","playback_object_key":"processed/vid_1/source.mp4"}`))
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestInternalIngestionCreatesFeedItem(t *testing.T) {
	app, _ := newFeedTestApp(t, testOptions{internalToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/v1/internal/feed-items", strings.NewReader(`{
		"event_id":"evt_1",
		"video_id":"vid_1",
		"owner_id":"usr_1",
		"title":"Ready video",
		"thumbnail_object_key":"thumbnails/vid_1/poster.jpg",
		"playback_object_key":"processed/vid_1/source.mp4",
		"duration_ms":1234,
		"ready_at":"2026-07-06T10:00:00Z"
	}`))
	req.Header.Set("X-Internal-Token", "secret")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body readyVideoEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Created || body.Data.VideoID != "vid_1" || body.Data.Title != "Ready video" {
		t.Fatalf("body = %#v", body)
	}
}

func newTestApp(readyErr error) http.Handler {
	if readyErr == nil {
		app, _ := newFeedTestApp(nil, testOptions{})
		return app
	}
	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	New(stubReady{err: readyErr}).RegisterRoutes(mux, metrics.Handler())
	var app http.Handler = mux
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(nil, app)
	return app
}

type testOptions struct {
	internalToken string
}

func newFeedTestApp(t *testing.T, options testOptions) (http.Handler, *service.FeedService) {
	if t != nil {
		t.Helper()
	}
	metrics := observability.NewMetrics()
	svc := service.NewFeedService(repository.NewMemoryStore(), service.Options{
		Metrics:      metrics,
		DefaultLimit: 2,
		MaxLimit:     2,
	})
	mux := http.NewServeMux()
	New(svc, Options{InternalAPIToken: options.internalToken}).RegisterRoutes(mux, metrics.Handler())
	var app http.Handler = mux
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(nil, app)
	return app, svc
}

func insertReadyVideo(t *testing.T, svc *service.FeedService, eventID string, videoID string, readyAt time.Time) {
	t.Helper()
	if _, _, err := svc.UpsertReadyVideo(context.Background(), domain.ReadyVideoInput{
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
	}); err != nil {
		t.Fatalf("UpsertReadyVideo() error = %v", err)
	}
}

type stubReady struct {
	err error
}

func (s stubReady) Ready(context.Context) error {
	return s.err
}

func (s stubReady) ListFeed(context.Context, service.FeedQuery) (service.FeedPage, error) {
	return service.FeedPage{}, s.err
}

func (s stubReady) UpsertReadyVideo(context.Context, domain.ReadyVideoInput) (domain.FeedItem, bool, error) {
	return domain.FeedItem{}, false, s.err
}
