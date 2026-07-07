package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/service"
)

func TestCreateStartEndLiveSessionHTTP(t *testing.T) {
	app := newTestApp()

	createReq := httptest.NewRequest(http.MethodPost, "/v1/live-sessions", strings.NewReader(`{"title":"Demo live","description":"test"}`))
	createReq.Header.Set("X-User-ID", "usr_creator")
	createReq.Header.Set("X-Request-ID", "req_create")
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Data struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			StreamKey string `json:"stream_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Data.ID == "" || created.Data.StreamKey == "" || created.Data.Status != "scheduled" {
		t.Fatalf("unexpected create response: %#v", created.Data)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/live-sessions/"+created.Data.ID+"/start", nil)
	startReq.Header.Set("X-User-ID", "usr_creator")
	startRec := httptest.NewRecorder()
	app.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %s", startRec.Code, startRec.Body.String())
	}

	endReq := httptest.NewRequest(http.MethodPost, "/v1/live-sessions/"+created.Data.ID+"/end", nil)
	endReq.Header.Set("X-User-ID", "usr_creator")
	endRec := httptest.NewRecorder()
	app.ServeHTTP(endRec, endReq)
	if endRec.Code != http.StatusAccepted {
		t.Fatalf("end status = %d, body = %s", endRec.Code, endRec.Body.String())
	}
}

func TestCreateRequiresTrustedUser(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/v1/live-sessions", strings.NewReader(`{"title":"Demo live"}`))
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "UNAUTHORIZED")
}

func TestListDoesNotReturnStreamKey(t *testing.T) {
	app := newTestApp()
	req := httptest.NewRequest(http.MethodPost, "/v1/live-sessions", strings.NewReader(`{"title":"Demo live"}`))
	req.Header.Set("X-User-ID", "usr_creator")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d", rec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/live-sessions", nil)
	listRec := httptest.NewRecorder()
	app.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), "stream_key") {
		t.Fatalf("list response leaked stream_key: %s", listRec.Body.String())
	}
}

func newTestApp() http.Handler {
	store := repository.NewMemoryStore()
	svc := service.NewLiveService(store, service.Options{
		DefaultLimit:    20,
		MaxLimit:        50,
		IngestBaseURL:   "rtmp://media.local/live",
		PlaybackBaseURL: "http://media.local/live",
		StreamKeyBytes:  24,
	})
	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	New(svc).RegisterRoutes(mux, metrics.Handler())
	return observability.RequestContextMiddleware(nil, mux)
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var decoded struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if decoded.Error.Code != want {
		t.Fatalf("error code = %q, want %q; body = %s", decoded.Error.Code, want, string(body))
	}
}
