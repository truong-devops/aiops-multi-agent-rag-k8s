package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func RequestContextMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		requestID := req.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = "req_" + randomHex(16)
		}
		req.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(req.Context(), requestIDKey, requestID)
		next.ServeHTTP(recorder, req.WithContext(ctx))

		logger.Info(
			"request completed",
			"service", "identity-service",
			"method", req.Method,
			"path", req.URL.Path,
			"status", recorder.status,
			"request_id", requestID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}
