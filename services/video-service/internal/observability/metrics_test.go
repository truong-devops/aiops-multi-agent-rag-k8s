package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsIncludesObjectVerificationAndStatusTransitions(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordObjectVerification("verified")
	metrics.RecordStatusTransition("uploaded", "processing", "updated")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `video_service_object_verifications_total{outcome="verified"} 1`) {
		t.Fatalf("missing object verification metric: %s", body)
	}
	if !strings.Contains(body, `video_service_status_transitions_total{from="uploaded",to="processing",outcome="updated"} 1`) {
		t.Fatalf("missing status transition metric: %s", body)
	}
}
