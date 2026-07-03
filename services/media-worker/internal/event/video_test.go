package event

import (
	"testing"
	"time"
)

func TestParseUploadedEvent(t *testing.T) {
	receivedAt := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	raw := []byte(`{
		"event_id":"evt_123",
		"event_name":"video.uploaded",
		"event_version":"v1",
		"event_type":"video.uploaded.v1",
		"aggregate_id":"vid_123",
		"producer":"video-service",
		"environment":"test",
		"correlation_id":"corr_123",
		"request_id":"req_123",
		"occurred_at":"2026-07-03T09:59:00Z",
		"payload":{
			"video_id":"vid_123",
			"owner_id":"usr_123",
			"raw_object_key":"raw/vid_123/source.mp4",
			"content_type":"video/mp4",
			"size_bytes":2048
		}
	}`)

	event, err := ParseUploadedEvent(raw, receivedAt)
	if err != nil {
		t.Fatalf("ParseUploadedEvent() error = %v", err)
	}
	if event.EventID != "evt_123" || event.VideoID != "vid_123" || event.RawObjectKey == "" {
		t.Fatalf("event = %#v", event)
	}
	if event.RequestID != "req_123" || event.CorrelationID != "corr_123" {
		t.Fatalf("trace ids = %s/%s", event.RequestID, event.CorrelationID)
	}
}

func TestParseUploadedEventRejectsWrongType(t *testing.T) {
	_, err := ParseUploadedEvent([]byte(`{
		"event_id":"evt_123",
		"event_type":"video.ready.v1",
		"payload":{}
	}`), time.Now())
	if err == nil {
		t.Fatal("ParseUploadedEvent() error = nil, want invalid type")
	}
}
