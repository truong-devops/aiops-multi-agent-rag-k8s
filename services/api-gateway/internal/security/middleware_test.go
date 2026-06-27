package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeVerifier struct {
	claims Claims
	err    error
	token  string
}

func (v *fakeVerifier) VerifyAccessToken(_ context.Context, token string) (Claims, error) {
	v.token = token
	if v.err != nil {
		return Claims{}, v.err
	}
	return v.claims, nil
}

func TestAuthMiddlewareRequiresTokenForProtectedPrefixes(t *testing.T) {
	handler := AuthMiddleware(&fakeVerifier{}, []string{"/api/v1/videos/"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/videos/vid_test", nil)
	req.Header.Set("X-Request-ID", "req_auth")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "AUTH_REQUIRED")
}

func TestAuthMiddlewareForwardsTrustedUserContext(t *testing.T) {
	verifier := &fakeVerifier{
		claims: Claims{
			Subject:   "usr_123",
			Email:     "user@example.com",
			Roles:     []string{"user", "admin"},
			SessionID: "sess_123",
		},
	}
	handler := AuthMiddleware(verifier, []string{"/api/v1/videos/"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-User-ID") != "usr_123" {
			t.Fatalf("X-User-ID = %q", req.Header.Get("X-User-ID"))
		}
		if req.Header.Get("X-User-Email") != "user@example.com" {
			t.Fatalf("X-User-Email = %q", req.Header.Get("X-User-Email"))
		}
		if req.Header.Get("X-User-Roles") != "user,admin" {
			t.Fatalf("X-User-Roles = %q", req.Header.Get("X-User-Roles"))
		}
		if req.Header.Get("X-Session-ID") != "sess_123" {
			t.Fatalf("X-Session-ID = %q", req.Header.Get("X-Session-ID"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/videos/upload-requests", nil)
	req.Header.Set("Authorization", "Bearer token_123")
	req.Header.Set("X-User-ID", "usr_spoofed")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if verifier.token != "token_123" {
		t.Fatalf("verified token = %q", verifier.token)
	}
}

func TestAuthMiddlewareAllowsPublicRouteAndRemovesSpoofedContext(t *testing.T) {
	handler := AuthMiddleware(&fakeVerifier{}, []string{"/api/v1/videos/"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-User-ID") != "" {
			t.Fatalf("X-User-ID = %q, want stripped", req.Header.Get("X-User-ID"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.Header.Set("X-User-ID", "usr_spoofed")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
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
