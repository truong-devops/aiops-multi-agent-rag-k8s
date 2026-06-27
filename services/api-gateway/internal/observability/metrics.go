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
	mu       sync.Mutex
	requests map[metricKey]uint64
	latency  map[metricKey]time.Duration
}

type metricKey struct {
	Method string
	Status int
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests: map[metricKey]uint64{},
		latency:  map[metricKey]time.Duration{},
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
	m.mu.Unlock()

	_, _ = fmt.Fprintln(w, "# HELP api_gateway_http_requests_total Total HTTP requests handled by api-gateway.")
	_, _ = fmt.Fprintln(w, "# TYPE api_gateway_http_requests_total counter")
	for _, key := range keys {
		_, _ = fmt.Fprintf(
			w,
			"api_gateway_http_requests_total{method=%q,status=%q} %d\n",
			key.Method,
			strconv.Itoa(key.Status),
			requests[key],
		)
	}

	_, _ = fmt.Fprintln(w, "# HELP api_gateway_http_request_duration_seconds_total Total HTTP request duration handled by api-gateway.")
	_, _ = fmt.Fprintln(w, "# TYPE api_gateway_http_request_duration_seconds_total counter")
	for _, key := range keys {
		_, _ = fmt.Fprintf(
			w,
			"api_gateway_http_request_duration_seconds_total{method=%q,status=%q} %.6f\n",
			key.Method,
			strconv.Itoa(key.Status),
			latency[key].Seconds(),
		)
	}
}
