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

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func CorrelationIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(correlationIDKey).(string)
	return value
}

func RequestContextMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		requestID := headerOrGenerated(req, "X-Request-ID")
		correlationID := headerOrGenerated(req, "X-Correlation-ID")
		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("X-Correlation-ID", correlationID)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Correlation-ID", correlationID)

		ctx := context.WithValue(req.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, correlationIDKey, correlationID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, req.WithContext(ctx))

		logger.Info(
			"request completed",
			"service", "live-service",
			"method", req.Method,
			"path", req.URL.Path,
			"status", recorder.status,
			"request_id", requestID,
			"correlation_id", correlationID,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	})
}

func BodyLimitMiddleware(limitBytes int64, next http.Handler) http.Handler {
	if limitBytes <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, limitBytes)
		next.ServeHTTP(w, req)
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
