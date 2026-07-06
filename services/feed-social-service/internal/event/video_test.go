package event

import (
	"testing"
	"time"
)

func TestParseReadyEvent(t *testing.T) {
	receivedAt := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	raw := []byte(`{
		"event_id":"evt_123",
		"event_name":"video.ready",
		"event_version":"v1",
		"event_type":"video.ready.v1",
		"aggregate_id":"vid_123",
		"producer":"media-worker",
		"environment":"test",
		"correlation_id":"corr_123",
		"request_id":"req_123",
		"occurred_at":"2026-07-06T09:59:00Z",
		"payload":{
			"video_id":"vid_123",
			"owner_id":"usr_123",
			"title":"Ready video",
			"processed_object_key":"processed/vid_123/source.mp4",
			"thumbnail_object_key":"thumbnails/vid_123/poster.jpg",
			"duration_ms":2048,
			"ready_at":"2026-07-06T09:59:30Z"
		}
	}`)

	input, occurredAt, err := ParseReadyEvent(raw, receivedAt)
	if err != nil {
		t.Fatalf("ParseReadyEvent() error = %v", err)
	}
	if input.EventID != "evt_123" ||
		input.VideoID != "vid_123" ||
		input.OwnerID != "usr_123" ||
		input.PlaybackObjectKey != "processed/vid_123/source.mp4" ||
		input.ThumbnailObjectKey != "thumbnails/vid_123/poster.jpg" {
		t.Fatalf("input = %#v", input)
	}
	if input.RequestID != "req_123" || input.CorrelationID != "corr_123" {
		t.Fatalf("trace ids = %s/%s", input.RequestID, input.CorrelationID)
	}
	if occurredAt.Format(time.RFC3339) != "2026-07-06T09:59:00Z" {
		t.Fatalf("occurredAt = %s", occurredAt)
	}
	if input.ReadyAt.Format(time.RFC3339) != "2026-07-06T09:59:30Z" {
		t.Fatalf("readyAt = %s", input.ReadyAt)
	}
}

func TestParseReadyEventRejectsWrongType(t *testing.T) {
	_, _, err := ParseReadyEvent([]byte(`{
		"event_id":"evt_123",
		"event_type":"video.uploaded.v1",
		"payload":{}
	}`), time.Now())
	if err == nil {
		t.Fatal("ParseReadyEvent() error = nil, want invalid type")
	}
}
