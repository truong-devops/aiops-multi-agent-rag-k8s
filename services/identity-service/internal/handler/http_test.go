package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/security"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/service"
)

func TestIdentityPasswordAuthFlow(t *testing.T) {
	app := newTestApp(t)

	registerResp := doJSON(t, app, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        "User@Example.com",
		"username":     "user01",
		"display_name": "User One",
		"password":     "strong-password",
	}, "")
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}
	assertJSONPath(t, registerResp.Body.Bytes(), "data.user.email", "user@example.com")

	duplicateResp := doJSON(t, app, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    "user@example.com",
		"password": "strong-password",
	}, "")
	if duplicateResp.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicateResp.Code, duplicateResp.Body.String())
	}
	assertErrorCode(t, duplicateResp.Body.Bytes(), "EMAIL_ALREADY_EXISTS")

	loginResp := doJSON(t, app, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "user@example.com",
		"password": "strong-password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}
	accessToken := mustString(t, loginResp.Body.Bytes(), "data.access_token")
	refreshToken := mustString(t, loginResp.Body.Bytes(), "data.refresh_token")
	if accessToken == "" || refreshToken == "" {
		t.Fatal("login response missing tokens")
	}

	meResp := doJSON(t, app, http.MethodGet, "/v1/users/me", nil, "Bearer "+accessToken)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}
	assertJSONPath(t, meResp.Body.Bytes(), "data.user.display_name", "User One")

	patchResp := doJSON(t, app, http.MethodPatch, "/v1/users/me", map[string]any{
		"display_name": "Updated Name",
		"avatar_url":   "https://example.com/avatar.png",
	}, "Bearer "+accessToken)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", patchResp.Code, patchResp.Body.String())
	}
	assertJSONPath(t, patchResp.Body.Bytes(), "data.user.display_name", "Updated Name")

	refreshResp := doJSON(t, app, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, "")
	if refreshResp.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", refreshResp.Code, refreshResp.Body.String())
	}
	newRefreshToken := mustString(t, refreshResp.Body.Bytes(), "data.refresh_token")
	if newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Fatalf("refresh token rotation failed old=%q new=%q", refreshToken, newRefreshToken)
	}

	reuseResp := doJSON(t, app, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, "")
	if reuseResp.Code != http.StatusUnauthorized {
		t.Fatalf("reuse status = %d, body = %s", reuseResp.Code, reuseResp.Body.String())
	}
	assertErrorCode(t, reuseResp.Body.Bytes(), "REFRESH_TOKEN_REUSED")

	meAfterReuseResp := doJSON(t, app, http.MethodGet, "/v1/users/me", nil, "Bearer "+accessToken)
	if meAfterReuseResp.Code != http.StatusUnauthorized {
		t.Fatalf("me after refresh reuse status = %d, body = %s", meAfterReuseResp.Code, meAfterReuseResp.Body.String())
	}

	logoutResp := doJSON(t, app, http.MethodPost, "/v1/auth/logout", map[string]any{
		"refresh_token": newRefreshToken,
	}, "")
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logoutResp.Code, logoutResp.Body.String())
	}
}

func TestIdentityLogoutRevokesAccessTokenSession(t *testing.T) {
	app := newTestApp(t)

	_ = doJSON(t, app, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    "logout@example.com",
		"password": "strong-password",
	}, "")

	loginResp := doJSON(t, app, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "logout@example.com",
		"password": "strong-password",
	}, "")
	accessToken := mustString(t, loginResp.Body.Bytes(), "data.access_token")
	refreshToken := mustString(t, loginResp.Body.Bytes(), "data.refresh_token")

	logoutResp := doJSON(t, app, http.MethodPost, "/v1/auth/logout", map[string]any{
		"refresh_token": refreshToken,
	}, "")
	if logoutResp.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logoutResp.Code, logoutResp.Body.String())
	}

	meResp := doJSON(t, app, http.MethodGet, "/v1/users/me", nil, "Bearer "+accessToken)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, body = %s", meResp.Code, meResp.Body.String())
	}
}

func TestIdentityConcurrentRefreshOnlyRotatesOnce(t *testing.T) {
	app := newTestApp(t)

	_ = doJSON(t, app, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    "race@example.com",
		"password": "strong-password",
	}, "")

	loginResp := doJSON(t, app, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "race@example.com",
		"password": "strong-password",
	}, "")
	refreshToken := mustString(t, loginResp.Body.Bytes(), "data.refresh_token")

	payload, err := json.Marshal(map[string]any{"refresh_token": refreshToken})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	const attempts = 2
	statuses := make(chan int, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			statuses <- rec.Code
		}()
	}
	wg.Wait()
	close(statuses)

	okCount := 0
	reuseCount := 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			okCount++
		case http.StatusUnauthorized:
			reuseCount++
		default:
			t.Fatalf("unexpected refresh status = %d", status)
		}
	}
	if okCount != 1 || reuseCount != 1 {
		t.Fatalf("concurrent refresh results ok=%d unauthorized=%d, want 1 and 1", okCount, reuseCount)
	}
}

func TestIdentityInvalidLoginAndUnauthorizedProfile(t *testing.T) {
	app := newTestApp(t)

	_ = doJSON(t, app, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    "user@example.com",
		"password": "strong-password",
	}, "")

	loginResp := doJSON(t, app, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "user@example.com",
		"password": "wrong-password",
	}, "")
	if loginResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}
	assertErrorCode(t, loginResp.Body.Bytes(), "INVALID_CREDENTIALS")

	meResp := doJSON(t, app, http.MethodGet, "/v1/users/me", nil, "")
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}
	assertErrorCode(t, meResp.Body.Bytes(), "UNAUTHORIZED")
}

func TestIdentityJWKSAndGoogleNotConfigured(t *testing.T) {
	app := newTestApp(t)

	jwksResp := doJSON(t, app, http.MethodGet, "/.well-known/jwks.json", nil, "")
	if jwksResp.Code != http.StatusOK {
		t.Fatalf("jwks status = %d, body = %s", jwksResp.Code, jwksResp.Body.String())
	}
	assertArrayNotEmpty(t, jwksResp.Body.Bytes(), "keys")

	googleResp := doJSON(t, app, http.MethodGet, "/v1/auth/google/start?redirect_uri=http://localhost:3000/callback", nil, "")
	if googleResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("google start status = %d, body = %s", googleResp.Code, googleResp.Body.String())
	}
	assertErrorCode(t, googleResp.Body.Bytes(), "GOOGLE_NOT_CONFIGURED")
}

func TestIdentityGoogleStartDoesNotExposeCodeVerifier(t *testing.T) {
	app := newTestAppWithGoogleConfig(t)

	resp := doJSON(t, app, http.MethodGet, "/v1/auth/google/start?redirect_uri=http://localhost:3000/callback", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("google start status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("google start response missing data object: %s", resp.Body.String())
	}
	if _, exists := data["code_verifier"]; exists {
		t.Fatalf("google start leaked code_verifier in body = %s", resp.Body.String())
	}
	if data["authorization_url"] == "" || data["state"] == "" {
		t.Fatalf("google start missing authorization_url or state: %s", resp.Body.String())
	}
}

func TestIdentityUnknownRouteReturnsJSONError(t *testing.T) {
	app := newTestApp(t)

	resp := doJSON(t, app, http.MethodGet, "/v1/unknown", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, body = %s", resp.Code, resp.Body.String())
	}
	assertErrorCode(t, resp.Body.Bytes(), "ROUTE_NOT_FOUND")
}

func TestIdentityReadinessFailureReturnsServiceUnavailable(t *testing.T) {
	app := newTestAppWithReadiness(t, func(context.Context) error {
		return errors.New("database is down")
	})

	resp := doJSON(t, app, http.MethodGet, "/readyz", nil, "")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, body = %s", resp.Code, resp.Body.String())
	}
	assertErrorCode(t, resp.Body.Bytes(), "SERVICE_NOT_READY")
}

func newTestApp(t *testing.T) http.Handler {
	t.Helper()

	return newTestAppWithGoogle(t, service.GoogleOAuthConfig{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		JWKSURL:  "https://www.googleapis.com/oauth2/v3/certs",
		Scopes:   []string{"openid", "email", "profile"},
	})
}

func newTestAppWithGoogleConfig(t *testing.T) http.Handler {
	t.Helper()

	return newTestAppWithGoogle(t, service.GoogleOAuthConfig{
		ClientID:     "google-client",
		ClientSecret: "google-secret",
		AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		JWKSURL:      "https://www.googleapis.com/oauth2/v3/certs",
		Scopes:       []string{"openid", "email", "profile"},
	})
}

func newTestAppWithGoogle(t *testing.T, googleConfig service.GoogleOAuthConfig) http.Handler {
	t.Helper()

	return newTestAppWithGoogleAndReadiness(t, googleConfig, nil)
}

func newTestAppWithReadiness(t *testing.T, readiness func(context.Context) error) http.Handler {
	t.Helper()

	return newTestAppWithGoogleAndReadiness(t, service.GoogleOAuthConfig{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
		JWKSURL:  "https://www.googleapis.com/oauth2/v3/certs",
		Scopes:   []string{"openid", "email", "profile"},
	}, readiness)
}

func newTestAppWithGoogleAndReadiness(t *testing.T, googleConfig service.GoogleOAuthConfig, readiness func(context.Context) error) http.Handler {
	t.Helper()

	store := repository.NewMemoryStore()
	jwtManager, err := security.NewJWTManager("aiops-video-platform", "aiops-api", "")
	if err != nil {
		t.Fatalf("NewJWTManager() error = %v", err)
	}
	auth := service.NewAuthService(store, jwtManager, 15*time.Minute, 7*24*time.Hour)
	google := service.NewGoogleOAuthService(store, googleConfig)
	mux := http.NewServeMux()
	New(auth, google, readiness).RegisterRoutes(mux)
	return observability.RequestContextMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil)), mux)
}

func doJSON(t *testing.T, app http.Handler, method string, path string, payload any, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("X-Request-ID", "req_test")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	assertJSONPath(t, body, "error.code", want)
}

func assertJSONPath(t *testing.T, body []byte, path string, want string) {
	t.Helper()
	got := mustString(t, body, path)
	if got != want {
		t.Fatalf("%s = %q, want %q; body = %s", path, got, want, string(body))
	}
}

func mustString(t *testing.T, body []byte, path string) string {
	t.Helper()

	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", string(body), err)
	}
	current := decoded
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s: current value is not object in body %s", path, string(body))
		}
		current = object[part]
	}
	value, ok := current.(string)
	if !ok {
		t.Fatalf("%s: value %v is not string in body %s", path, current, string(body))
	}
	return value
}

func assertArrayNotEmpty(t *testing.T, body []byte, path string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", string(body), err)
	}
	value, ok := decoded[path].([]any)
	if !ok || len(value) == 0 {
		t.Fatalf("%s is empty or not array in body %s", path, string(body))
	}
}
