package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

type contextKey string

const (
	requestIDKey     contextKey = "request_id"
	correlationIDKey contextKey = "correlation_id"
)

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

func CORSMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" && originAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Request-ID, X-Correlation-ID, Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func originAllowed(origin string, allowedOrigins []string) bool {
	return slices.Contains(allowedOrigins, "*") || slices.Contains(allowedOrigins, origin)
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
