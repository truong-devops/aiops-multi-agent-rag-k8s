package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/observability"
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
	if !strings.Contains(rec.Body.String(), "media_worker_http_requests_total") {
		t.Fatalf("metrics body = %s", rec.Body.String())
	}
}

func newTestApp(readyErr error) http.Handler {
	metrics := observability.NewMetrics()
	mux := http.NewServeMux()
	New(stubReady{err: readyErr}).RegisterRoutes(mux, metrics.Handler())
	var app http.Handler = mux
	app = metrics.Middleware(app)
	app = observability.RequestContextMiddleware(nil, app)
	return app
}

type stubReady struct {
	err error
}

func (s stubReady) Ready(context.Context) error {
	return s.err
}
