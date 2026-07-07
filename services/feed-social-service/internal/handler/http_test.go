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

func TestLikeRequiresTrustedUserContext(t *testing.T) {
	app, svc := newFeedTestApp(t, testOptions{})
	insertReadyVideo(t, svc, "evt_like_auth", "vid_like_auth", time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC))
	req := httptest.NewRequest(http.MethodPut, "/v1/videos/vid_like_auth/like", nil)
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestLikeIsIdempotentAndUpdatesCounters(t *testing.T) {
	app, svc := newFeedTestApp(t, testOptions{})
	insertReadyVideo(t, svc, "evt_like", "vid_like", time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/v1/videos/vid_like/like", nil)
		req.Header.Set("X-User-ID", "usr_123")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("like status = %d, body = %s", rec.Code, rec.Body.String())
		}
	}

	socialReq := httptest.NewRequest(http.MethodGet, "/v1/videos/vid_like/social", nil)
	socialRec := httptest.NewRecorder()
	app.ServeHTTP(socialRec, socialReq)
	var body socialEnvelope
	if err := json.Unmarshal(socialRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode social response: %v", err)
	}
	if body.Data.LikeCount != 1 {
		t.Fatalf("like_count = %d, want 1", body.Data.LikeCount)
	}

	unlikeReq := httptest.NewRequest(http.MethodDelete, "/v1/videos/vid_like/like", nil)
	unlikeReq.Header.Set("X-User-ID", "usr_123")
	unlikeRec := httptest.NewRecorder()
	app.ServeHTTP(unlikeRec, unlikeReq)
	if unlikeRec.Code != http.StatusOK {
		t.Fatalf("unlike status = %d, body = %s", unlikeRec.Code, unlikeRec.Body.String())
	}
}

func TestCommentLifecycle(t *testing.T) {
	app, svc := newFeedTestApp(t, testOptions{})
	insertReadyVideo(t, svc, "evt_comment", "vid_comment", time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC))

	createReq := httptest.NewRequest(http.MethodPost, "/v1/videos/vid_comment/comments", strings.NewReader(`{"body":"hello feed"}`))
	createReq.Header.Set("X-User-ID", "usr_123")
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created commentEnvelope
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode comment response: %v", err)
	}
	if created.Data.ID == "" || created.Data.Body != "hello feed" || created.Counters.CommentCount != 1 {
		t.Fatalf("created = %#v", created)
	}

	listRec := httptest.NewRecorder()
	app.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/videos/vid_comment/comments", nil))
	var listed commentsEnvelope
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode comments response: %v", err)
	}
	if len(listed.Data) != 1 || listed.Data[0].ID != created.Data.ID {
		t.Fatalf("listed = %#v", listed)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/comments/"+created.Data.ID, nil)
	deleteReq.Header.Set("X-User-ID", "usr_123")
	deleteRec := httptest.NewRecorder()
	app.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
	var deleted deleteCommentEnvelope
	if err := json.Unmarshal(deleteRec.Body.Bytes(), &deleted); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if !deleted.Deleted || deleted.Data.Body != "" || deleted.Counters.CommentCount != 0 {
		t.Fatalf("deleted = %#v", deleted)
	}
}

func TestFollowLifecycle(t *testing.T) {
	app, _ := newFeedTestApp(t, testOptions{})
	req := httptest.NewRequest(http.MethodPut, "/v1/users/usr_creator/follow", nil)
	req.Header.Set("X-User-ID", "usr_viewer")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("follow status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var followed followEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &followed); err != nil {
		t.Fatalf("decode follow response: %v", err)
	}
	if !followed.Following || !followed.Changed || followed.Data.FolloweeID != "usr_creator" {
		t.Fatalf("followed = %#v", followed)
	}

	unfollowReq := httptest.NewRequest(http.MethodDelete, "/v1/users/usr_creator/follow", nil)
	unfollowReq.Header.Set("X-User-ID", "usr_viewer")
	unfollowRec := httptest.NewRecorder()
	app.ServeHTTP(unfollowRec, unfollowReq)
	if unfollowRec.Code != http.StatusOK {
		t.Fatalf("unfollow status = %d, body = %s", unfollowRec.Code, unfollowRec.Body.String())
	}
}

func TestFollowRejectsSelfFollow(t *testing.T) {
	app, _ := newFeedTestApp(t, testOptions{})
	req := httptest.NewRequest(http.MethodPut, "/v1/users/usr_viewer/follow", nil)
	req.Header.Set("X-User-ID", "usr_viewer")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
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

func (s stubReady) GetSocialCounters(context.Context, string) (domain.VideoSocialCounters, error) {
	return domain.VideoSocialCounters{}, s.err
}

func (s stubReady) LikeVideo(context.Context, string, service.Actor, string, string) (domain.VideoSocialCounters, bool, error) {
	return domain.VideoSocialCounters{}, false, s.err
}

func (s stubReady) UnlikeVideo(context.Context, string, service.Actor, string, string) (domain.VideoSocialCounters, bool, error) {
	return domain.VideoSocialCounters{}, false, s.err
}

func (s stubReady) CreateComment(context.Context, service.CreateCommentInput) (domain.Comment, domain.VideoSocialCounters, error) {
	return domain.Comment{}, domain.VideoSocialCounters{}, s.err
}

func (s stubReady) ListComments(context.Context, service.CommentQuery) (service.CommentPage, error) {
	return service.CommentPage{}, s.err
}

func (s stubReady) DeleteComment(context.Context, string, service.Actor) (domain.Comment, domain.VideoSocialCounters, bool, error) {
	return domain.Comment{}, domain.VideoSocialCounters{}, false, s.err
}

func (s stubReady) FollowUser(context.Context, string, service.Actor, string, string) (domain.Follow, bool, error) {
	return domain.Follow{}, false, s.err
}

func (s stubReady) UnfollowUser(context.Context, string, service.Actor, string, string) (domain.Follow, bool, error) {
	return domain.Follow{}, false, s.err
}

func (s stubReady) UpsertReadyVideo(context.Context, domain.ReadyVideoInput) (domain.FeedItem, bool, error) {
	return domain.FeedItem{}, false, s.err
}
