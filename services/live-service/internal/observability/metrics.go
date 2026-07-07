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
	liveOperations map[operationMetricKey]uint64
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
		liveOperations: map[operationMetricKey]uint64{},
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

func (m *Metrics) RecordLiveOperation(operation string, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.liveOperations[key]++
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
	requests := copyHTTP(m.requests)
	requestLatency := copyHTTPDuration(m.requestLatency)
	dbKeys := sortedOperationKeys(m.dbOperations)
	dbOperations := copyOperation(m.dbOperations)
	dbLatency := copyOperationDuration(m.dbLatency)
	liveKeys := sortedOperationKeys(m.liveOperations)
	liveOperations := copyOperation(m.liveOperations)
	m.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP live_http_requests_total Total HTTP requests handled by live-service.")
	_, _ = fmt.Fprintln(w, "# TYPE live_http_requests_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "live_http_requests_total{method=%q,status=%q} %d\n", key.Method, strconv.Itoa(key.Status), requests[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP live_http_request_duration_seconds_total Total HTTP request duration handled by live-service.")
	_, _ = fmt.Fprintln(w, "# TYPE live_http_request_duration_seconds_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "live_http_request_duration_seconds_total{method=%q,status=%q} %.6f\n", key.Method, strconv.Itoa(key.Status), requestLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP live_db_operations_total Database operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE live_db_operations_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "live_db_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, dbOperations[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP live_db_operation_duration_seconds_total Total database operation duration by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE live_db_operation_duration_seconds_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "live_db_operation_duration_seconds_total{operation=%q,outcome=%q} %.6f\n", key.Operation, key.Outcome, dbLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP live_operations_total Live session operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE live_operations_total counter")
	for _, key := range liveKeys {
		_, _ = fmt.Fprintf(w, "live_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, liveOperations[key])
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

func copyHTTP(values map[httpMetricKey]uint64) map[httpMetricKey]uint64 {
	out := make(map[httpMetricKey]uint64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func copyHTTPDuration(values map[httpMetricKey]time.Duration) map[httpMetricKey]time.Duration {
	out := make(map[httpMetricKey]time.Duration, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func copyOperation(values map[operationMetricKey]uint64) map[operationMetricKey]uint64 {
	out := make(map[operationMetricKey]uint64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func copyOperationDuration(values map[operationMetricKey]time.Duration) map[operationMetricKey]time.Duration {
	out := make(map[operationMetricKey]time.Duration, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func normalizeLabel(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
