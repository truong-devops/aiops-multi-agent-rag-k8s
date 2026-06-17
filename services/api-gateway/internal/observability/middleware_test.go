package observability

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestContextMiddlewarePropagatesRequestIDs(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Request-ID") != "req_existing" {
			t.Fatalf("request X-Request-ID = %q", req.Header.Get("X-Request-ID"))
		}
		if req.Header.Get("X-Correlation-ID") != "corr_existing" {
			t.Fatalf("request X-Correlation-ID = %q", req.Header.Get("X-Correlation-ID"))
		}
		w.WriteHeader(http.StatusAccepted)
	})
	handler := RequestContextMiddleware(testLogger(), next)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req_existing")
	req.Header.Set("X-Correlation-ID", "corr_existing")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") != "req_existing" {
		t.Fatalf("response X-Request-ID = %q", rec.Header().Get("X-Request-ID"))
	}
	if rec.Header().Get("X-Correlation-ID") != "corr_existing" {
		t.Fatalf("response X-Correlation-ID = %q", rec.Header().Get("X-Correlation-ID"))
	}
}

func TestRequestContextMiddlewareGeneratesRequestIDs(t *testing.T) {
	handler := RequestContextMiddleware(testLogger(), http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Request-ID") == "" {
			t.Fatal("missing generated X-Request-ID")
		}
		if req.Header.Get("X-Correlation-ID") == "" {
			t.Fatal("missing generated X-Correlation-ID")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing response X-Request-ID")
	}
	if rec.Header().Get("X-Correlation-ID") == "" {
		t.Fatal("missing response X-Correlation-ID")
	}
}

func TestCORSMiddlewareHandlesAllowedPreflight(t *testing.T) {
	handler := CORSMiddleware([]string{"http://localhost:3000"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Fatal("next handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/videos", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareDoesNotAllowUnknownOrigin(t *testing.T) {
	handler := CORSMiddleware([]string{"http://localhost:3000"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/videos", nil)
	req.Header.Set("Origin", "http://evil.local")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBodyLimitMiddlewareLimitsRequestBody(t *testing.T) {
	handler := BodyLimitMiddleware(4, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, err := io.ReadAll(req.Body); err == nil {
			t.Fatal("ReadAll() error = nil, want body too large")
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/videos", strings.NewReader("12345"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
