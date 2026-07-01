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
	mu                  sync.Mutex
	requests            map[metricKey]uint64
	latency             map[metricKey]time.Duration
	uploadRequests      map[string]uint64
	uploadConfirmations map[string]uint64
	presignOperations   map[string]uint64
	outboxOperations    map[string]uint64
	dbOperations        map[operationMetricKey]uint64
	dbLatency           map[operationMetricKey]time.Duration
}

type metricKey struct {
	Method string
	Status int
}

type operationMetricKey struct {
	Operation string
	Outcome   string
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:            map[metricKey]uint64{},
		latency:             map[metricKey]time.Duration{},
		uploadRequests:      map[string]uint64{},
		uploadConfirmations: map[string]uint64{},
		presignOperations:   map[string]uint64{},
		outboxOperations:    map[string]uint64{},
		dbOperations:        map[operationMetricKey]uint64{},
		dbLatency:           map[operationMetricKey]time.Duration{},
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, req)
		m.Record(req.Method, recorder.status, time.Since(startedAt))
	})
}

func (m *Metrics) Record(method string, status int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := metricKey{Method: method, Status: status}
	m.requests[key]++
	m.latency[key] += duration
}

func (m *Metrics) RecordUploadRequest(outcome string) {
	m.recordNamed(m.uploadRequests, normalizeOutcome(outcome))
}

func (m *Metrics) RecordUploadConfirmation(outcome string) {
	m.recordNamed(m.uploadConfirmations, normalizeOutcome(outcome))
}

func (m *Metrics) RecordPresign(outcome string) {
	m.recordNamed(m.presignOperations, normalizeOutcome(outcome))
}

func (m *Metrics) RecordOutboxPublish(outcome string) {
	m.recordNamed(m.outboxOperations, normalizeOutcome(outcome))
}

func (m *Metrics) RecordDBOperation(operation string, outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeOutcome(outcome)}
	m.dbOperations[key]++
	m.dbLatency[key] += duration
}

func (m *Metrics) recordNamed(values map[string]uint64, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	values[outcome]++
}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.writePrometheus(w)
	}
}

func (m *Metrics) writePrometheus(w http.ResponseWriter) {
	m.mu.Lock()
	keys := make([]metricKey, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method == keys[j].Method {
			return keys[i].Status < keys[j].Status
		}
		return keys[i].Method < keys[j].Method
	})
	requests := make(map[metricKey]uint64, len(m.requests))
	latency := make(map[metricKey]time.Duration, len(m.latency))
	for _, key := range keys {
		requests[key] = m.requests[key]
		latency[key] = m.latency[key]
	}
	uploadRequests := copyStringCounters(m.uploadRequests)
	uploadConfirmations := copyStringCounters(m.uploadConfirmations)
	presignOperations := copyStringCounters(m.presignOperations)
	outboxOperations := copyStringCounters(m.outboxOperations)
	dbKeys := make([]operationMetricKey, 0, len(m.dbOperations))
	for key := range m.dbOperations {
		dbKeys = append(dbKeys, key)
	}
	sort.Slice(dbKeys, func(i, j int) bool {
		if dbKeys[i].Operation == dbKeys[j].Operation {
			return dbKeys[i].Outcome < dbKeys[j].Outcome
		}
		return dbKeys[i].Operation < dbKeys[j].Operation
	})
	dbOperations := make(map[operationMetricKey]uint64, len(m.dbOperations))
	dbLatency := make(map[operationMetricKey]time.Duration, len(m.dbLatency))
	for _, key := range dbKeys {
		dbOperations[key] = m.dbOperations[key]
		dbLatency[key] = m.dbLatency[key]
	}
	m.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP video_service_http_requests_total Total HTTP requests handled by video-service.")
	_, _ = fmt.Fprintln(w, "# TYPE video_service_http_requests_total counter")
	for _, key := range keys {
		_, _ = fmt.Fprintf(w, "video_service_http_requests_total{method=%q,status=%q} %d\n", key.Method, strconv.Itoa(key.Status), requests[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP video_service_http_request_duration_seconds_total Total HTTP request duration handled by video-service.")
	_, _ = fmt.Fprintln(w, "# TYPE video_service_http_request_duration_seconds_total counter")
	for _, key := range keys {
		_, _ = fmt.Fprintf(w, "video_service_http_request_duration_seconds_total{method=%q,status=%q} %.6f\n", key.Method, strconv.Itoa(key.Status), latency[key].Seconds())
	}

	writeOutcomeCounter(w, "video_service_upload_requests_total", "Upload request creation attempts by outcome.", uploadRequests)
	writeOutcomeCounter(w, "video_service_upload_confirmations_total", "Upload confirmation attempts by outcome.", uploadConfirmations)
	writeOutcomeCounter(w, "video_service_presign_operations_total", "Presigned upload URL generation attempts by outcome.", presignOperations)
	writeOutcomeCounter(w, "video_service_outbox_publish_total", "Outbox publish attempts by outcome.", outboxOperations)

	_, _ = fmt.Fprintln(w, "# HELP video_service_db_operations_total Database operations by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE video_service_db_operations_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "video_service_db_operations_total{operation=%q,outcome=%q} %d\n", key.Operation, key.Outcome, dbOperations[key])
	}
	_, _ = fmt.Fprintln(w, "# HELP video_service_db_operation_duration_seconds_total Total database operation duration by operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE video_service_db_operation_duration_seconds_total counter")
	for _, key := range dbKeys {
		_, _ = fmt.Fprintf(w, "video_service_db_operation_duration_seconds_total{operation=%q,outcome=%q} %.6f\n", key.Operation, key.Outcome, dbLatency[key].Seconds())
	}
}

func copyStringCounters(values map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func writeOutcomeCounter(w http.ResponseWriter, name string, help string, values map[string]uint64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	_, _ = fmt.Fprintf(w, "# TYPE %s counter\n", name)
	outcomes := make([]string, 0, len(values))
	for outcome := range values {
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)
	for _, outcome := range outcomes {
		_, _ = fmt.Fprintf(w, "%s{outcome=%q} %d\n", name, outcome, values[outcome])
	}
}

func normalizeOutcome(outcome string) string {
	return normalizeLabel(outcome, "unknown")
}

func normalizeLabel(value string, fallback string) string {
	value = strconv.Quote(value)
	value, _ = strconv.Unquote(value)
	if value == "" {
		return fallback
	}
	return value
}
