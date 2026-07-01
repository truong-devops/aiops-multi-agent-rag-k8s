package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/service"
)

func TestVideoUploadFlow(t *testing.T) {
	app := newTestApp()

	createBody := bytes.NewBufferString(`{
		"title": "Launch video",
		"description": "demo",
		"visibility": "public",
		"content_type": "video/mp4",
		"size_bytes": 1000
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/videos/upload-requests", createBody)
	createReq.Header.Set("X-User-ID", "usr_123")
	createReq.Header.Set("X-Request-ID", "req_create")
	createReq.Header.Set("X-Correlation-ID", "corr_video")
	createRec := httptest.NewRecorder()

	app.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Data struct {
			Video struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"video"`
			UploadRequest struct {
				ID        string `json:"id"`
				ObjectKey string `json:"object_key"`
			} `json:"upload_request"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	decodeJSONResponse(t, createRec.Body.Bytes(), &created)
	if created.RequestID != "req_create" {
		t.Fatalf("request_id = %q", created.RequestID)
	}
	if created.Data.Video.ID == "" || created.Data.Video.Status != "draft" {
		t.Fatalf("created video = %#v", created.Data.Video)
	}
	if created.Data.UploadRequest.ID == "" || !strings.HasPrefix(created.Data.UploadRequest.ObjectKey, "raw/") {
		t.Fatalf("upload request = %#v", created.Data.UploadRequest)
	}

	confirmReq := httptest.NewRequest(
		http.MethodPost,
		"/v1/videos/"+created.Data.Video.ID+"/uploaded",
		bytes.NewBufferString(`{"upload_request_id":"`+created.Data.UploadRequest.ID+`","size_bytes":1200}`),
	)
	confirmReq.Header.Set("X-Request-ID", "req_confirm")
	confirmReq.Header.Set("X-Correlation-ID", "corr_video")
	confirmReq.Header.Set("X-User-ID", "usr_123")
	confirmRec := httptest.NewRecorder()

	app.ServeHTTP(confirmRec, confirmReq)

	if confirmRec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", confirmRec.Code, confirmRec.Body.String())
	}
	var confirmed struct {
		Data struct {
			Video struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				SizeBytes int64  `json:"size_bytes"`
			} `json:"video"`
		} `json:"data"`
	}
	decodeJSONResponse(t, confirmRec.Body.Bytes(), &confirmed)
	if confirmed.Data.Video.Status != "uploaded" || confirmed.Data.Video.SizeBytes != 1200 {
		t.Fatalf("confirmed video = %#v", confirmed.Data.Video)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/videos/"+created.Data.Video.ID, nil)
	getReq.Header.Set("X-User-ID", "usr_123")
	getRec := httptest.NewRecorder()
	app.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
}

func TestCreateUploadRequestRequiresUserContext(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/v1/videos/upload-requests", bytes.NewBufferString(`{"title":"x","content_type":"video/mp4"}`))
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "UNAUTHORIZED")
}

func TestListVideos(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodGet, "/v1/videos?limit=10", nil)
	req.Header.Set("X-User-ID", "usr_123")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"page"`) {
		t.Fatalf("missing page body: %s", rec.Body.String())
	}
}

func newTestApp() http.Handler {
	store := repository.NewMemoryStore()
	videoService := service.NewVideoService(store, service.Options{
		RawVideoBucket:   "raw-videos",
		UploadURLBase:    "http://minio.local",
		UploadRequestTTL: time.Hour,
	})
	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	New(videoService).RegisterRoutes(mux, metrics.Handler())

	var app http.Handler = mux
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(nil, app)
	return app
}

func decodeJSONResponse(t *testing.T, body []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decode response %q: %v", string(body), err)
	}
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var decoded struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSONResponse(t, body, &decoded)
	if decoded.Error.Code != want {
		t.Fatalf("error code = %q, want %q; body = %s", decoded.Error.Code, want, string(body))
	}
}
