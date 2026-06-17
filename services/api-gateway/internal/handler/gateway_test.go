package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/config"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/api-gateway/internal/observability"
)

func TestGatewayRoutesAndRewritesPublicAPIPrefix(t *testing.T) {
	type upstreamRequest struct {
		path           string
		rawQuery       string
		requestID      string
		correlationID  string
		gateway        string
		forwardedHost  string
		forwardedProto string
		method         string
		body           string
		authorization  string
		idempotencyKey string
		contentType    string
		accept         string
	}

	seen := make(chan upstreamRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		seen <- upstreamRequest{
			path:           req.URL.Path,
			rawQuery:       req.URL.RawQuery,
			requestID:      req.Header.Get("X-Request-ID"),
			correlationID:  req.Header.Get("X-Correlation-ID"),
			gateway:        req.Header.Get("X-Gateway"),
			forwardedHost:  req.Header.Get("X-Forwarded-Host"),
			forwardedProto: req.Header.Get("X-Forwarded-Proto"),
			method:         req.Method,
			body:           string(body),
			authorization:  req.Header.Get("Authorization"),
			idempotencyKey: req.Header.Get("Idempotency-Key"),
			contentType:    req.Header.Get("Content-Type"),
			accept:         req.Header.Get("Accept"),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer upstream.Close()

	gateway := newTestGateway(t, []config.Route{
		{Name: "identity-service", Prefix: "/api/v1/auth/", Target: mustURL(t, upstream.URL)},
	})
	app := observability.RequestContextMiddleware(testLogger(), gateway)

	body := `{"email":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login?return_to=home", strings.NewReader(body))
	req.Host = "api.local"
	req.Header.Set("X-Request-ID", "req_test")
	req.Header.Set("X-Correlation-ID", "corr_test")
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Idempotency-Key", "idem_test")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") != "req_test" {
		t.Fatalf("response X-Request-ID = %q", rec.Header().Get("X-Request-ID"))
	}

	got := <-seen
	if got.path != "/v1/auth/login" {
		t.Fatalf("upstream path = %q, want /v1/auth/login", got.path)
	}
	if got.rawQuery != "return_to=home" {
		t.Fatalf("upstream query = %q, want return_to=home", got.rawQuery)
	}
	if got.method != http.MethodPost || got.body != body {
		t.Fatalf("upstream method/body = %s %q", got.method, got.body)
	}
	if got.requestID != "req_test" || got.correlationID != "corr_test" {
		t.Fatalf("forwarded request ids = %q %q", got.requestID, got.correlationID)
	}
	if got.gateway != "api-gateway" {
		t.Fatalf("X-Gateway = %q, want api-gateway", got.gateway)
	}
	if got.forwardedHost != "api.local" {
		t.Fatalf("X-Forwarded-Host = %q, want api.local", got.forwardedHost)
	}
	if got.forwardedProto != "http" {
		t.Fatalf("X-Forwarded-Proto = %q, want http", got.forwardedProto)
	}
	if got.authorization != "Bearer token" || got.idempotencyKey != "idem_test" {
		t.Fatalf("important headers not forwarded: authorization=%q idem=%q", got.authorization, got.idempotencyKey)
	}
	if got.contentType != "application/json" || got.accept != "application/json" {
		t.Fatalf("content negotiation headers not forwarded: content-type=%q accept=%q", got.contentType, got.accept)
	}
}

func TestGatewayMatchesFeedRouteWithoutMatchingFeedback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte(req.URL.Path))
	}))
	defer upstream.Close()

	gateway := newTestGateway(t, []config.Route{
		{Name: "feed-social-service", Prefix: "/api/v1/feed", Target: mustURL(t, upstream.URL)},
	})

	for _, path := range []string{"/api/v1/feed", "/api/v1/feed/items"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		gateway.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if !strings.HasPrefix(rec.Body.String(), "/v1/feed") {
			t.Fatalf("%s upstream path = %q, want /v1/feed prefix", path, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/feedback", nil)
	rec := httptest.NewRecorder()

	gateway.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("feedback status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "ROUTE_NOT_FOUND")
}

func TestGatewayReturnsJSONBadGatewayWhenUpstreamFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()

	gateway := newTestGateway(t, []config.Route{
		{Name: "identity-service", Prefix: "/api/v1/auth/", Target: mustURL(t, "http://"+addr)},
	})
	app := observability.RequestContextMiddleware(testLogger(), gateway)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	req.Header.Set("X-Request-ID", "req_failed")
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502, body = %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", contentType)
	}
	assertErrorCode(t, rec.Body.Bytes(), "UPSTREAM_UNAVAILABLE")
}

func TestNewGatewayRejectsInvalidRoutes(t *testing.T) {
	if _, err := NewGateway([]config.Route{{Prefix: "/v1/auth/", Target: mustURL(t, "http://example.com")}}, time.Second, testLogger()); err == nil {
		t.Fatal("NewGateway() error = nil, want invalid prefix error")
	}
	if _, err := NewGateway([]config.Route{{Prefix: "/api/v1/auth/"}}, time.Second, testLogger()); err == nil {
		t.Fatal("NewGateway() error = nil, want invalid target error")
	}
}

func newTestGateway(t *testing.T, routes []config.Route) *Gateway {
	t.Helper()
	gateway, err := NewGateway(routes, time.Second, testLogger())
	if err != nil {
		t.Fatalf("NewGateway() error = %v", err)
	}
	return gateway
}

func mustURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	return parsed
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()

	var decoded struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode error body %q: %v", string(body), err)
	}
	if decoded.Error.Code != want {
		t.Fatalf("error code = %q, want %q; body = %s", decoded.Error.Code, want, string(body))
	}
}
