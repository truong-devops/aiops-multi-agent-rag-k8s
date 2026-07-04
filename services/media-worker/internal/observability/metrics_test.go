package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsRenderOperationalEvidence(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordJobStatus("queued", 2)
	metrics.RecordQueueState("processing", 2, 15*time.Second)
	metrics.RecordAttemptOutcome("dead_letter", "FFMPEG_FAILED")
	metrics.RecordDependencyOperation("minio", "download_object", "error", 50*time.Millisecond)
	metrics.RecordDependencyOperation("video-service", "update_status", "ready", 20*time.Millisecond)
	metrics.RecordEventAge("video.uploaded", "created", 3*time.Second)

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`media_worker_jobs_by_status{status="queued"} 2`,
		`media_worker_queue_depth{queue="processing"} 2`,
		`media_worker_queue_oldest_runnable_age_seconds{queue="processing"} 15.000000`,
		`media_worker_attempt_outcomes_total{outcome="dead_letter",error_code="FFMPEG_FAILED"} 1`,
		`media_worker_dependency_operations_total{dependency="minio",operation="download_object",outcome="error"} 1`,
		`media_worker_dependency_operations_total{dependency="video-service",operation="update_status",outcome="ready"} 1`,
		`media_worker_event_age_observations_total{source="video.uploaded",outcome="created"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, body)
		}
	}
}
