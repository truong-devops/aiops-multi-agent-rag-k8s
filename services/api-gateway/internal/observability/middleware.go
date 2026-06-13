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

const (
	requestIDKey     contextKey = "request_id"
	correlationIDKey contextKey = "correlation_id"
)

func RequestContextMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		requestID := headerOrGenerated(req, "X-Request-ID")
		correlationID := headerOrGenerated(req, "X-Correlation-ID")

		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("X-Correlation-ID", correlationID)

		ctx := context.WithValue(req.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, correlationIDKey, correlationID)

		next.ServeHTTP(w, req.WithContext(ctx))

		logger.Info(
			"request completed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", requestID,
			"correlation_id", correlationID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

func headerOrGenerated(req *http.Request, header string) string {
	if value := req.Header.Get(header); value != "" {
		return value
	}
	return "req_" + randomHex(16)
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}
