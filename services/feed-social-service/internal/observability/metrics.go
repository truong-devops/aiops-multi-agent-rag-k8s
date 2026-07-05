package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Metrics struct {
	mu             sync.Mutex
	requests       map[httpMetricKey]uint64
	requestLatency map[httpMetricKey]time.Duration
	dbOperations   map[operationMetricKey]uint64
	dbLatency      map[operationMetricKey]time.Duration
	feedOperations map[operationMetricKey]uint64
}

type httpMetricKey struct {
	Method string
	Status int
}

type operationMetricKey struct {
	Operation string
	Outcome   string
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:       map[httpMetricKey]uint64{},
		requestLatency: map[httpMetricKey]time.Duration{},
		dbOperations:   map[operationMetricKey]uint64{},
		dbLatency:      map[operationMetricKey]time.Duration{},
		feedOperations: map[operationMetricKey]uint64{},
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, req)
		m.RecordHTTP(req.Method, recorder.status, time.Since(startedAt))
	})
}

func (m *Metrics) RecordHTTP(method string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := httpMetricKey{Method: normalizeLabel(method, "UNKNOWN"), Status: status}
	m.requests[key]++
	m.requestLatency[key] += duration
}

func (m *Metrics) RecordDBOperation(operation string, outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.dbOperations[key]++
	m.dbLatency[key] += duration
}

func (m *Metrics) RecordFeedOperation(operation string, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.feedOperations[key]++
}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.writePrometheus(w)
	}
}

func (m *Metrics) writePrometheus(w http.ResponseWriter) {
	m.mu.Lock()
	httpKeys := sortedHTTPKeys(m.requests)
	requests := make(map[httpMetricKey]uint64, len(m.requests))
	requestLatency := make(map[httpMetricKey]time.Duration, len(m.requestLatency))
	for _, key := range httpKeys {
		requests[key] = m.requests[key]
		requestLatency[key] = m.requestLatency[key]
	}
	dbKeys := sortedOperationKeys(m.dbOperations)
	dbOperations := make(map[operationMetricKey]uint64, len(m.dbOperations))
	dbLatency := make(map[operationMetricKey]time.Duration, len(m.dbLatency))
	for _, key := range dbKeys {
		dbOperations[key] = m.dbOperations[key]
		dbLatency[key] = m.dbLatency[key]
	}
	feedKeys := sortedOperationKeys(m.feedOperations)
	feedOperations := make(map[operationMetricKey]uint64, len(m.feedOperations))
	for _, key := range feedKeys {
		feedOperations[key] = m.feedOperations[key]
	}
	m.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP feed_social_http_requests_total Total HTTP requests handled by feed-social-service.")
	_, _ = fmt.Fprintln(w, "# TYPE feed_social_http_requests_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "feed_social_http_requests_total{method=%q,status=%q} %d\n", key.Method, strconv.Itoa(key.Status), requests[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP feed_social_http_request_duration_seconds_total Total HTTP request duration handled by feed-social-service.")
	_, _ = fmt.Fprintln(w, "# TYPE feed_social_http_request_duration_seconds_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "feed_social_http_request_duration_seconds_total{method=%q,status=%q} %.6f\n", key.Method, strconv.Itoa(key.Status), requestLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP feed_social_db_operations_total Database operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE feed_social_db_operations_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "feed_social_db_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, dbOperations[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP feed_social_db_operation_duration_seconds_total Total database operation duration by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE feed_social_db_operation_duration_seconds_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "feed_social_db_operation_duration_seconds_total{operation=%q,outcome=%q} %.6f\n", key.Operation, key.Outcome, dbLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP feed_social_operations_total Feed/social operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE feed_social_operations_total counter")
	for _, key := range feedKeys {
		_, _ = fmt.Fprintf(w, "feed_social_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, feedOperations[key])
	}
}

func sortedHTTPKeys(values map[httpMetricKey]uint64) []httpMetricKey {
	keys := make([]httpMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method == keys[j].Method {
			return keys[i].Status < keys[j].Status
		}
		return keys[i].Method < keys[j].Method
	})
	return keys
}

func sortedOperationKeys(values map[operationMetricKey]uint64) []operationMetricKey {
	keys := make([]operationMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Operation == keys[j].Operation {
			return keys[i].Outcome < keys[j].Outcome
		}
		return keys[i].Operation < keys[j].Operation
	})
	return keys
}

func normalizeLabel(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
