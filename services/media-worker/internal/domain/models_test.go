package domain

import (
	"testing"
	"time"
)

func TestCanTransitionJob(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{JobStatusQueued, JobStatusRunning, true},
		{JobStatusRunning, JobStatusSucceeded, true},
		{JobStatusRunning, JobStatusRetrying, true},
		{JobStatusRetrying, JobStatusRunning, true},
		{JobStatusSucceeded, JobStatusRunning, false},
	}
	for _, tt := range tests {
		if got := CanTransitionJob(tt.from, tt.to); got != tt.want {
			t.Fatalf("CanTransitionJob(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestNewProcessingJobFromUploadedEvent(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	job, err := NewProcessingJobFromUploadedEvent(UploadedVideoEvent{
		EventID:       "evt_123",
		VideoID:       "vid_123",
		OwnerID:       "usr_123",
		RawObjectKey:  "raw/vid_123/source.mp4",
		ContentType:   "video/mp4",
		SizeBytes:     1024,
		RequestID:     "req_123",
		CorrelationID: "corr_123",
	}, "raw-videos", 3, now)
	if err != nil {
		t.Fatalf("NewProcessingJobFromUploadedEvent() error = %v", err)
	}
	if job.Status != JobStatusQueued || job.VideoID != "vid_123" || job.InputBucket != "raw-videos" {
		t.Fatalf("job = %#v", job)
	}
	if job.NextRunAt != now || job.MaxAttempts != 3 {
		t.Fatalf("job schedule = %s/%d", job.NextRunAt, job.MaxAttempts)
	}
}
