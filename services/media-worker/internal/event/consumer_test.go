package event

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

func TestUploadedConsumerHandleCreatesJobAndCommits(t *testing.T) {
	consumer := &fakeConsumer{}
	service := &fakeUploadedService{}
	worker := NewUploadedConsumerWorker(UploadedConsumerConfig{
		Consumer: consumer,
		Service:  service,
		Now:      func() time.Time { return time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC) },
	})
	message := Message{Value: []byte(`{
		"event_id":"evt_123",
		"event_type":"video.uploaded.v1",
		"request_id":"req_123",
		"correlation_id":"corr_123",
		"payload":{
			"video_id":"vid_123",
			"owner_id":"usr_123",
			"raw_object_key":"raw/vid_123/source.mp4",
			"content_type":"video/mp4",
			"size_bytes":1024
		}
	}`)}

	if err := worker.Handle(context.Background(), message); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if service.calls != 1 {
		t.Fatalf("service calls = %d, want 1", service.calls)
	}
	if consumer.commits != 1 {
		t.Fatalf("commits = %d, want 1", consumer.commits)
	}
}

func TestUploadedConsumerHandleCommitsInvalidEvent(t *testing.T) {
	consumer := &fakeConsumer{}
	service := &fakeUploadedService{}
	worker := NewUploadedConsumerWorker(UploadedConsumerConfig{Consumer: consumer, Service: service})

	if err := worker.Handle(context.Background(), Message{Value: []byte(`{"event_id":"evt_bad","event_type":"video.ready.v1","payload":{}}`)}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if service.calls != 0 {
		t.Fatalf("service calls = %d, want 0", service.calls)
	}
	if consumer.commits != 1 {
		t.Fatalf("commits = %d, want 1", consumer.commits)
	}
}

type fakeConsumer struct {
	commits int
}

func (f *fakeConsumer) Fetch(context.Context) (Message, error) {
	return Message{}, context.Canceled
}

func (f *fakeConsumer) Commit(context.Context, Message) error {
	f.commits++
	return nil
}

func (f *fakeConsumer) Close() error {
	return nil
}

type fakeUploadedService struct {
	calls int
}

func (f *fakeUploadedService) RegisterUploadedEvent(_ context.Context, event domain.UploadedVideoEvent) (domain.ProcessingJob, bool, error) {
	f.calls++
	return domain.ProcessingJob{ID: "job_123", VideoID: event.VideoID}, true, nil
}
