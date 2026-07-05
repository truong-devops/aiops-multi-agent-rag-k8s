package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsRenderFeedAndDBEvidence(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordDBOperation("list_feed_items", "success", 25*time.Millisecond)
	metrics.RecordFeedOperation("upsert_ready_video", "created")

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`feed_social_db_operations_total{operation="list_feed_items",outcome="success"} 1`,
		`feed_social_operations_total{operation="upsert_ready_video",outcome="created"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, body)
		}
	}
}
