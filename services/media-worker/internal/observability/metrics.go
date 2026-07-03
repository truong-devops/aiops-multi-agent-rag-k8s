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
	mu            sync.Mutex
	requests      map[httpMetricKey]uint64
	latency       map[httpMetricKey]time.Duration
	jobOperations map[operationMetricKey]uint64
	dbOperations  map[operationMetricKey]uint64
	dbLatency     map[operationMetricKey]time.Duration
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
		requests:      map[httpMetricKey]uint64{},
		latency:       map[httpMetricKey]time.Duration{},
		jobOperations: map[operationMetricKey]uint64{},
		dbOperations:  map[operationMetricKey]uint64{},
		dbLatency:     map[operationMetricKey]time.Duration{},
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
	key := httpMetricKey{Method: method, Status: status}
	m.requests[key]++
	m.latency[key] += duration
}

func (m *Metrics) RecordJobOperation(operation string, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.jobOperations[key]++
}

func (m *Metrics) RecordDBOperation(operation string, outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.dbOperations[key]++
	m.dbLatency[key] += duration
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
	latency := make(map[httpMetricKey]time.Duration, len(m.latency))
	for _, key := range httpKeys {
		requests[key] = m.requests[key]
		latency[key] = m.latency[key]
	}
	jobKeys := sortedOperationKeys(m.jobOperations)
	jobOperations := make(map[operationMetricKey]uint64, len(m.jobOperations))
	for _, key := range jobKeys {
		jobOperations[key] = m.jobOperations[key]
	}
	dbKeys := sortedOperationKeys(m.dbOperations)
	dbOperations := make(map[operationMetricKey]uint64, len(m.dbOperations))
	dbLatency := make(map[operationMetricKey]time.Duration, len(m.dbLatency))
	for _, key := range dbKeys {
		dbOperations[key] = m.dbOperations[key]
		dbLatency[key] = m.dbLatency[key]
	}
	m.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP media_worker_http_requests_total Total HTTP requests handled by media-worker.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_http_requests_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "media_worker_http_requests_total{method=%q,status=%q} %d\n", key.Method, strconv.Itoa(key.Status), requests[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_http_request_duration_seconds_total Total HTTP request duration handled by media-worker.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_http_request_duration_seconds_total counter")
	for _, key := range httpKeys {
		_, _ = fmt.Fprintf(w, "media_worker_http_request_duration_seconds_total{method=%q,status=%q} %.6f\n", key.Method, strconv.Itoa(key.Status), latency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_job_operations_total Job operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_job_operations_total counter")
	for _, key := range jobKeys {
		_, _ = fmt.Fprintf(w, "media_worker_job_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, jobOperations[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_db_operations_total Database operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_db_operations_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "media_worker_db_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, dbOperations[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_db_operation_duration_seconds_total Total database operation duration by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_db_operation_duration_seconds_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "media_worker_db_operation_duration_seconds_total{operation=%q,outcome=%q} %.6f\n", key.Operation, key.Outcome, dbLatency[key].Seconds())
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
