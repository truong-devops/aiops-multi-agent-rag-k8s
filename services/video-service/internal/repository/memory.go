package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type MemoryStore struct {
	mu            sync.RWMutex
	videos        map[string]domain.Video
	uploads       map[string]domain.UploadRequest
	statusHistory []domain.StatusHistory
	outboxEvents  []domain.OutboxEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		videos:  map[string]domain.Video{},
		uploads: map[string]domain.UploadRequest{},
	}
}

func (s *MemoryStore) CreateVideoWithUploadRequest(_ context.Context, video domain.Video, upload domain.UploadRequest, history domain.StatusHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos[video.ID] = video
	s.uploads[upload.ID] = upload
	s.statusHistory = append(s.statusHistory, history)
	return nil
}

func (s *MemoryStore) FindVideoByID(_ context.Context, id string) (domain.Video, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	video, ok := s.videos[id]
	if !ok {
		return domain.Video{}, domain.NotFound(domain.CodeVideoNotFound, "Video was not found.")
	}
	return video, nil
}

func (s *MemoryStore) ListVideos(_ context.Context, filter ListVideosFilter) ([]domain.Video, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	videos := make([]domain.Video, 0, len(s.videos))
	for _, video := range s.videos {
		if filter.OwnerID != "" && video.OwnerID != filter.OwnerID {
			continue
		}
		if filter.Status != "" && video.Status != filter.Status {
			continue
		}
		videos = append(videos, video)
	}
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].CreatedAt.After(videos[j].CreatedAt)
	})
	if filter.Limit > 0 && len(videos) > filter.Limit {
		videos = videos[:filter.Limit]
	}
	return videos, nil
}

func (s *MemoryStore) FindUploadRequestByID(_ context.Context, id string) (domain.UploadRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	upload, ok := s.uploads[id]
	if !ok {
		return domain.UploadRequest{}, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found.")
	}
	return upload, nil
}

func (s *MemoryStore) FindUploadIntentByIdempotencyKey(_ context.Context, ownerID string, idempotencyKey string) (domain.Video, domain.UploadRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, upload := range s.uploads {
		if upload.OwnerID != ownerID || upload.IdempotencyKey != idempotencyKey {
			continue
		}
		video, ok := s.videos[upload.VideoID]
		if !ok {
			return domain.Video{}, domain.UploadRequest{}, domain.NotFound(domain.CodeVideoNotFound, "Video was not found.")
		}
		return video, upload, nil
	}
	return domain.Video{}, domain.UploadRequest{}, domain.NotFound(domain.CodeUploadRequestNotFound, "Upload request was not found.")
}

func (s *MemoryStore) SaveUploadRequest(_ context.Context, upload domain.UploadRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[upload.ID] = upload
	return nil
}

func (s *MemoryStore) CompleteUpload(_ context.Context, upload domain.UploadRequest, video domain.Video, history domain.StatusHistory, outbox domain.OutboxEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[upload.ID] = upload
	s.videos[video.ID] = video
	s.statusHistory = append(s.statusHistory, history)
	if outbox.ID != "" {
		s.outboxEvents = append(s.outboxEvents, outbox)
	}
	return nil
}

func (s *MemoryStore) SaveVideoStatus(_ context.Context, video domain.Video, history domain.StatusHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos[video.ID] = video
	s.statusHistory = append(s.statusHistory, history)
	return nil
}

func (s *MemoryStore) Ping(context.Context) error {
	return nil
}

func (s *MemoryStore) OutboxEvents() []domain.OutboxEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]domain.OutboxEvent, len(s.outboxEvents))
	copy(events, s.outboxEvents)
	return events
}

func (s *MemoryStore) ListPendingOutboxEvents(_ context.Context, limit int) ([]domain.OutboxEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 25
	}
	events := make([]domain.OutboxEvent, 0, limit)
	for _, event := range s.outboxEvents {
		if event.Status != domain.OutboxStatusPending {
			continue
		}
		events = append(events, event)
		if len(events) >= limit {
			break
		}
	}
	return events, nil
}

func (s *MemoryStore) MarkOutboxPublished(_ context.Context, id string, publishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, event := range s.outboxEvents {
		if event.ID != id {
			continue
		}
		event.Status = domain.OutboxStatusPublished
		event.PublishedAt = &publishedAt
		s.outboxEvents[index] = event
		return nil
	}
	return domain.NotFound(domain.CodeVideoNotFound, "Outbox event was not found.")
}

func (s *MemoryStore) MarkOutboxFailed(_ context.Context, id string, errMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, event := range s.outboxEvents {
		if event.ID != id {
			continue
		}
		event.Status = domain.OutboxStatusFailed
		event.Attempts++
		event.LastError = errMessage
		s.outboxEvents[index] = event
		return nil
	}
	return domain.NotFound(domain.CodeVideoNotFound, "Outbox event was not found.")
}
