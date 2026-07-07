package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]domain.LiveSession
	keys     map[string]domain.StreamKey
	events   map[string][]domain.LiveEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: map[string]domain.LiveSession{},
		keys:     map[string]domain.StreamKey{},
		events:   map[string][]domain.LiveEvent{},
	}
}

func (s *MemoryStore) CreateSession(_ context.Context, session domain.LiveSession, key domain.StreamKey, event domain.LiveEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	s.keys[key.LiveSessionID] = key
	s.events[session.ID] = append(s.events[session.ID], event)
	return nil
}

func (s *MemoryStore) FindSessionByID(_ context.Context, id string) (domain.LiveSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return domain.LiveSession{}, domain.NotFound(domain.CodeLiveSessionNotFound, "Live session was not found.")
	}
	return session, nil
}

func (s *MemoryStore) ListSessions(_ context.Context, filter ListSessionsFilter) ([]domain.LiveSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := normalizedLimit(filter.Limit)
	sessions := make([]domain.LiveSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		if filter.CreatorID != "" && session.CreatorID != filter.CreatorID {
			continue
		}
		if filter.Status != "" && session.Status != filter.Status {
			continue
		}
		if filter.BeforeCreatedAt != nil && !sessionBeforeCursor(session, *filter.BeforeCreatedAt, filter.BeforeSessionID) {
			continue
		}
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].CreatedAt.Equal(sessions[j].CreatedAt) {
			return sessions[i].ID > sessions[j].ID
		}
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func (s *MemoryStore) UpdateSessionState(_ context.Context, session domain.LiveSession, event domain.LiveEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[session.ID]; !ok {
		return domain.NotFound(domain.CodeLiveSessionNotFound, "Live session was not found.")
	}
	s.sessions[session.ID] = session
	s.events[session.ID] = append(s.events[session.ID], event)
	return nil
}

func (s *MemoryStore) Ping(_ context.Context) error {
	return nil
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 101 {
		return 101
	}
	return limit
}

func sessionBeforeCursor(session domain.LiveSession, before time.Time, beforeID string) bool {
	beforeID = strings.TrimSpace(beforeID)
	if session.CreatedAt.Before(before) {
		return true
	}
	if session.CreatedAt.Equal(before) && beforeID != "" {
		return session.ID < beforeID
	}
	return false
}
