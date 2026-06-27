package security

import (
	"encoding/json"
	"net/http"
	"strings"
)

var internalUserHeaders = []string{
	"X-User-ID",
	"X-User-Email",
	"X-User-Roles",
	"X-Session-ID",
}

func AuthMiddleware(verifier Verifier, requiredPrefixes []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		removeInternalHeaders(req)

		required := requiresAuth(req.URL.Path, requiredPrefixes)
		rawToken := bearerToken(req.Header.Get("Authorization"))
		if rawToken == "" {
			if required {
				writeAuthError(w, req, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication is required.")
				return
			}
			next.ServeHTTP(w, req)
			return
		}

		claims, err := verifier.VerifyAccessToken(req.Context(), rawToken)
		if err != nil {
			writeAuthError(w, req, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "Access token is invalid or expired.")
			return
		}

		req.Header.Set("X-User-ID", claims.Subject)
		req.Header.Set("X-User-Email", claims.Email)
		req.Header.Set("X-User-Roles", strings.Join(claims.Roles, ","))
		req.Header.Set("X-Session-ID", claims.SessionID)
		next.ServeHTTP(w, req)
	})
}

func removeInternalHeaders(req *http.Request) {
	for _, header := range internalUserHeaders {
		req.Header.Del(header)
	}
}

func requiresAuth(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasSuffix(prefix, "/") {
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func writeAuthError(w http.ResponseWriter, req *http.Request, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		} `json:"error"`
		RequestID string `json:"request_id,omitempty"`
	}{
		Error: struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		}{
			Code:    code,
			Message: message,
			Details: map[string]string{},
		},
		RequestID: req.Header.Get("X-Request-ID"),
	})
}
