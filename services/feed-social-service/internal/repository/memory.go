package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type MemoryStore struct {
	mu          sync.RWMutex
	feedItems   map[string]domain.FeedItem
	counters    map[string]domain.VideoSocialCounters
	inboxEvents map[string]domain.InboxEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		feedItems:   map[string]domain.FeedItem{},
		counters:    map[string]domain.VideoSocialCounters{},
		inboxEvents: map[string]domain.InboxEvent{},
	}
}

func (s *MemoryStore) UpsertFeedItemFromReadyVideo(_ context.Context, input domain.ReadyVideoInput, item domain.FeedItem) (domain.FeedItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.EventID != "" {
		if _, exists := s.inboxEvents[input.EventID]; exists {
			existing, ok := s.feedItems[item.VideoID]
			if !ok {
				return domain.FeedItem{}, false, domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
			}
			return existing, false, nil
		}
		processedAt := item.UpdatedAt
		s.inboxEvents[input.EventID] = domain.InboxEvent{
			ID:            input.EventID,
			EventName:     "video.ready",
			EventVersion:  "v1",
			AggregateID:   item.VideoID,
			Status:        domain.InboxStatusProcessed,
			RequestID:     input.RequestID,
			CorrelationID: input.CorrelationID,
			ReceivedAt:    input.ReceivedAt,
			ProcessedAt:   &processedAt,
		}
	}
	_, existed := s.feedItems[item.VideoID]
	s.feedItems[item.VideoID] = item
	if _, ok := s.counters[item.VideoID]; !ok {
		s.counters[item.VideoID] = domain.VideoSocialCounters{VideoID: item.VideoID, UpdatedAt: item.UpdatedAt}
	}
	return item, !existed, nil
}

func (s *MemoryStore) FindFeedItemByVideoID(_ context.Context, videoID string) (domain.FeedItemWithCounters, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.feedItems[videoID]
	if !ok {
		return domain.FeedItemWithCounters{}, domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	return domain.FeedItemWithCounters{Item: item, Counters: s.counters[videoID]}, nil
}

func (s *MemoryStore) ListFeedItems(_ context.Context, filter ListFeedFilter) ([]domain.FeedItemWithCounters, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := normalizedLimit(filter.Limit)
	items := make([]domain.FeedItem, 0, len(s.feedItems))
	for _, item := range s.feedItems {
		if item.Status != domain.FeedItemStatusActive {
			continue
		}
		if filter.BeforeReadyAt != nil && !itemBeforeCursor(item, *filter.BeforeReadyAt, filter.BeforeVideoID) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ReadyAt.Equal(items[j].ReadyAt) {
			return items[i].VideoID > items[j].VideoID
		}
		return items[i].ReadyAt.After(items[j].ReadyAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]domain.FeedItemWithCounters, 0, len(items))
	for _, item := range items {
		out = append(out, domain.FeedItemWithCounters{Item: item, Counters: s.counters[item.VideoID]})
	}
	return out, nil
}

func (s *MemoryStore) Ping(context.Context) error {
	return nil
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func itemBeforeCursor(item domain.FeedItem, beforeReadyAt time.Time, beforeVideoID string) bool {
	beforeReadyAt = beforeReadyAt.UTC()
	if item.ReadyAt.Before(beforeReadyAt) {
		return true
	}
	if item.ReadyAt.Equal(beforeReadyAt) && beforeVideoID != "" {
		return item.VideoID < beforeVideoID
	}
	return false
}
