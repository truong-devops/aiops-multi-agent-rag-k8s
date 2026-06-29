package repository

import (
	"context"
	"sort"
	"sync"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type MemoryStore struct {
	mu            sync.RWMutex
	videos        map[string]domain.Video
	uploads       map[string]domain.UploadRequest
	statusHistory []domain.StatusHistory
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

func (s *MemoryStore) SaveUploadRequest(_ context.Context, upload domain.UploadRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[upload.ID] = upload
	return nil
}

func (s *MemoryStore) CompleteUpload(_ context.Context, upload domain.UploadRequest, video domain.Video, history domain.StatusHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[upload.ID] = upload
	s.videos[video.ID] = video
	s.statusHistory = append(s.statusHistory, history)
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
