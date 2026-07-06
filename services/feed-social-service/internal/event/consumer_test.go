package event

import (
	"context"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

func TestReadyConsumerHandleCreatesFeedItemAndCommits(t *testing.T) {
	consumer := &fakeConsumer{}
	service := &fakeReadyService{}
	worker := NewReadyConsumerWorker(ReadyConsumerConfig{
		Consumer: consumer,
		Service:  service,
		Now:      func() time.Time { return time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC) },
	})
	message := Message{Value: []byte(`{
		"event_id":"evt_123",
		"event_type":"video.ready.v1",
		"request_id":"req_123",
		"correlation_id":"corr_123",
		"payload":{
			"video_id":"vid_123",
			"owner_id":"usr_123",
			"processed_object_key":"processed/vid_123/source.mp4",
			"thumbnail_object_key":"thumbnails/vid_123/poster.jpg"
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

func TestReadyConsumerHandleCommitsInvalidEvent(t *testing.T) {
	consumer := &fakeConsumer{}
	service := &fakeReadyService{}
	worker := NewReadyConsumerWorker(ReadyConsumerConfig{Consumer: consumer, Service: service})

	if err := worker.Handle(context.Background(), Message{Value: []byte(`{"event_id":"evt_bad","event_type":"video.uploaded.v1","payload":{}}`)}); err != nil {
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

type fakeReadyService struct {
	calls int
}

func (f *fakeReadyService) UpsertReadyVideo(_ context.Context, input domain.ReadyVideoInput) (domain.FeedItem, bool, error) {
	f.calls++
	return domain.FeedItem{VideoID: input.VideoID, OwnerID: input.OwnerID}, true, nil
}
