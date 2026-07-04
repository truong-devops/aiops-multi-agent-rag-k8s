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
	mu                sync.Mutex
	requests          map[httpMetricKey]uint64
	latency           map[httpMetricKey]time.Duration
	jobOperations     map[operationMetricKey]uint64
	jobStatus         map[string]int64
	queueDepth        map[string]int64
	queueOldestAge    map[string]time.Duration
	attemptOutcomes   map[attemptMetricKey]uint64
	dbOperations      map[operationMetricKey]uint64
	dbLatency         map[operationMetricKey]time.Duration
	dependencyOps     map[dependencyMetricKey]uint64
	dependencyLatency map[dependencyMetricKey]time.Duration
	eventAgeCount     map[operationMetricKey]uint64
	eventAgeLatency   map[operationMetricKey]time.Duration
}

type httpMetricKey struct {
	Method string
	Status int
}

type operationMetricKey struct {
	Operation string
	Outcome   string
}

type attemptMetricKey struct {
	Outcome   string
	ErrorCode string
}

type dependencyMetricKey struct {
	Dependency string
	Operation  string
	Outcome    string
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:          map[httpMetricKey]uint64{},
		latency:           map[httpMetricKey]time.Duration{},
		jobOperations:     map[operationMetricKey]uint64{},
		jobStatus:         map[string]int64{},
		queueDepth:        map[string]int64{},
		queueOldestAge:    map[string]time.Duration{},
		attemptOutcomes:   map[attemptMetricKey]uint64{},
		dbOperations:      map[operationMetricKey]uint64{},
		dbLatency:         map[operationMetricKey]time.Duration{},
		dependencyOps:     map[dependencyMetricKey]uint64{},
		dependencyLatency: map[dependencyMetricKey]time.Duration{},
		eventAgeCount:     map[operationMetricKey]uint64{},
		eventAgeLatency:   map[operationMetricKey]time.Duration{},
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

func (m *Metrics) RecordJobStatus(status string, count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobStatus[normalizeLabel(status, "unknown")] = count
}

func (m *Metrics) RecordQueueState(queue string, depth int64, oldestAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := normalizeLabel(queue, "default")
	m.queueDepth[key] = depth
	m.queueOldestAge[key] = oldestAge
}

func (m *Metrics) RecordAttemptOutcome(outcome string, errorCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := attemptMetricKey{
		Outcome:   normalizeLabel(outcome, "unknown"),
		ErrorCode: normalizeLabel(errorCode, "none"),
	}
	m.attemptOutcomes[key]++
}

func (m *Metrics) RecordDBOperation(operation string, outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(operation, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.dbOperations[key]++
	m.dbLatency[key] += duration
}

func (m *Metrics) RecordDependencyOperation(dependency string, operation string, outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := dependencyMetricKey{
		Dependency: normalizeLabel(dependency, "unknown"),
		Operation:  normalizeLabel(operation, "unknown"),
		Outcome:    normalizeLabel(outcome, "unknown"),
	}
	m.dependencyOps[key]++
	m.dependencyLatency[key] += duration
}

func (m *Metrics) RecordEventAge(source string, outcome string, age time.Duration) {
	if age < 0 {
		age = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := operationMetricKey{Operation: normalizeLabel(source, "unknown"), Outcome: normalizeLabel(outcome, "unknown")}
	m.eventAgeCount[key]++
	m.eventAgeLatency[key] += age
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
	jobStatusKeys := sortedStringKeys(m.jobStatus)
	jobStatus := copyInt64Map(m.jobStatus)
	queueKeys := sortedStringKeys(m.queueDepth)
	queueDepth := copyInt64Map(m.queueDepth)
	queueOldestAge := copyDurationByStringMap(m.queueOldestAge)
	attemptKeys := sortedAttemptKeys(m.attemptOutcomes)
	attemptOutcomes := make(map[attemptMetricKey]uint64, len(m.attemptOutcomes))
	for _, key := range attemptKeys {
		attemptOutcomes[key] = m.attemptOutcomes[key]
	}
	dbKeys := sortedOperationKeys(m.dbOperations)
	dbOperations := make(map[operationMetricKey]uint64, len(m.dbOperations))
	dbLatency := make(map[operationMetricKey]time.Duration, len(m.dbLatency))
	for _, key := range dbKeys {
		dbOperations[key] = m.dbOperations[key]
		dbLatency[key] = m.dbLatency[key]
	}
	dependencyKeys := sortedDependencyKeys(m.dependencyOps)
	dependencyOps := make(map[dependencyMetricKey]uint64, len(m.dependencyOps))
	dependencyLatency := make(map[dependencyMetricKey]time.Duration, len(m.dependencyLatency))
	for _, key := range dependencyKeys {
		dependencyOps[key] = m.dependencyOps[key]
		dependencyLatency[key] = m.dependencyLatency[key]
	}
	eventAgeKeys := sortedOperationKeys(m.eventAgeCount)
	eventAgeCount := make(map[operationMetricKey]uint64, len(m.eventAgeCount))
	eventAgeLatency := make(map[operationMetricKey]time.Duration, len(m.eventAgeLatency))
	for _, key := range eventAgeKeys {
		eventAgeCount[key] = m.eventAgeCount[key]
		eventAgeLatency[key] = m.eventAgeLatency[key]
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

	_, _ = fmt.Fprintln(w, "# HELP media_worker_jobs_by_status Current processing jobs by status.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_jobs_by_status gauge")
	for _, status := range jobStatusKeys {
		_, _ = fmt.Fprintf(w, "media_worker_jobs_by_status{status=%q} %d\n", status, jobStatus[status])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_queue_depth Current runnable processing job count by queue.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_queue_depth gauge")
	for _, queue := range queueKeys {
		_, _ = fmt.Fprintf(w, "media_worker_queue_depth{queue=%q} %d\n", queue, queueDepth[queue])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_queue_oldest_runnable_age_seconds Current age of the oldest runnable processing job by queue.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_queue_oldest_runnable_age_seconds gauge")
	for _, queue := range queueKeys {
		_, _ = fmt.Fprintf(w, "media_worker_queue_oldest_runnable_age_seconds{queue=%q} %.6f\n", queue, queueOldestAge[queue].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_attempt_outcomes_total Processing attempt outcomes by outcome and error code.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_attempt_outcomes_total counter")
	for _, key := range attemptKeys {
		_, _ = fmt.Fprintf(w, "media_worker_attempt_outcomes_total{outcome=%q,error_code=%q} %d\n", key.Outcome, key.ErrorCode, attemptOutcomes[key])
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

	_, _ = fmt.Fprintln(w, "# HELP media_worker_dependency_operations_total External dependency operations by dependency, operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_dependency_operations_total counter")
	for _, key := range dependencyKeys {
		_, _ = fmt.Fprintf(w, "media_worker_dependency_operations_total{dependency=%q,operation=%q,outcome=%q} %d\n", key.Dependency, key.Operation, key.Outcome, dependencyOps[key])
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_dependency_operation_duration_seconds_total Total external dependency operation duration by dependency, operation and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_dependency_operation_duration_seconds_total counter")
	for _, key := range dependencyKeys {
		_, _ = fmt.Fprintf(w, "media_worker_dependency_operation_duration_seconds_total{dependency=%q,operation=%q,outcome=%q} %.6f\n", key.Dependency, key.Operation, key.Outcome, dependencyLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_event_age_seconds_total Total observed event age by source and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_event_age_seconds_total counter")
	for _, key := range eventAgeKeys {
		_, _ = fmt.Fprintf(w, "media_worker_event_age_seconds_total{source=%q,outcome=%q} %.6f\n", key.Operation, key.Outcome, eventAgeLatency[key].Seconds())
	}

	_, _ = fmt.Fprintln(w, "# HELP media_worker_event_age_observations_total Observed event age samples by source and outcome.")
	_, _ = fmt.Fprintln(w, "# TYPE media_worker_event_age_observations_total counter")
	for _, key := range eventAgeKeys {
		_, _ = fmt.Fprintf(w, "media_worker_event_age_observations_total{source=%q,outcome=%q} %d\n", key.Operation, key.Outcome, eventAgeCount[key])
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

func sortedAttemptKeys(values map[attemptMetricKey]uint64) []attemptMetricKey {
	keys := make([]attemptMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Outcome == keys[j].Outcome {
			return keys[i].ErrorCode < keys[j].ErrorCode
		}
		return keys[i].Outcome < keys[j].Outcome
	})
	return keys
}

func sortedDependencyKeys(values map[dependencyMetricKey]uint64) []dependencyMetricKey {
	keys := make([]dependencyMetricKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Dependency == keys[j].Dependency {
			if keys[i].Operation == keys[j].Operation {
				return keys[i].Outcome < keys[j].Outcome
			}
			return keys[i].Operation < keys[j].Operation
		}
		return keys[i].Dependency < keys[j].Dependency
	})
	return keys
}

func sortedStringKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyInt64Map(values map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func copyDurationByStringMap(values map[string]time.Duration) map[string]time.Duration {
	out := make(map[string]time.Duration, len(values))
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
