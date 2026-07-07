package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type MemoryStore struct {
	mu          sync.RWMutex
	feedItems   map[string]domain.FeedItem
	counters    map[string]domain.VideoSocialCounters
	likes       map[string]domain.Like
	comments    map[string]domain.Comment
	follows     map[string]domain.Follow
	inboxEvents map[string]domain.InboxEvent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		feedItems:   map[string]domain.FeedItem{},
		counters:    map[string]domain.VideoSocialCounters{},
		likes:       map[string]domain.Like{},
		comments:    map[string]domain.Comment{},
		follows:     map[string]domain.Follow{},
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

func (s *MemoryStore) GetSocialCounters(_ context.Context, videoID string) (domain.VideoSocialCounters, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureActiveFeedItemLocked(videoID); err != nil {
		return domain.VideoSocialCounters{}, err
	}
	return s.counters[videoID], nil
}

func (s *MemoryStore) SetVideoLike(_ context.Context, mutation SocialMutation, liked bool) (domain.VideoSocialCounters, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureActiveFeedItemLocked(mutation.VideoID); err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	now := mutation.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	key := likeKey(mutation.VideoID, mutation.UserID)
	existing, exists := s.likes[key]
	changed := false
	if liked {
		if !exists {
			s.likes[key] = domain.Like{
				ID:            domain.NewID("like"),
				VideoID:       strings.TrimSpace(mutation.VideoID),
				UserID:        strings.TrimSpace(mutation.UserID),
				Status:        domain.LikeStatusActive,
				RequestID:     mutation.RequestID,
				CorrelationID: mutation.CorrelationID,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			changed = true
		} else if existing.Status != domain.LikeStatusActive {
			existing.Status = domain.LikeStatusActive
			existing.RequestID = mutation.RequestID
			existing.CorrelationID = mutation.CorrelationID
			existing.UpdatedAt = now
			s.likes[key] = existing
			changed = true
		}
		if changed {
			counter := s.counters[mutation.VideoID]
			counter.LikeCount++
			counter.UpdatedAt = now
			s.counters[mutation.VideoID] = counter
		}
		return s.counters[mutation.VideoID], changed, nil
	}
	if exists && existing.Status == domain.LikeStatusActive {
		existing.Status = domain.LikeStatusDeleted
		existing.RequestID = mutation.RequestID
		existing.CorrelationID = mutation.CorrelationID
		existing.UpdatedAt = now
		s.likes[key] = existing
		counter := s.counters[mutation.VideoID]
		if counter.LikeCount > 0 {
			counter.LikeCount--
		}
		counter.UpdatedAt = now
		s.counters[mutation.VideoID] = counter
		changed = true
	}
	return s.counters[mutation.VideoID], changed, nil
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

func (s *MemoryStore) CreateComment(_ context.Context, comment domain.Comment) (domain.Comment, domain.VideoSocialCounters, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureActiveFeedItemLocked(comment.VideoID); err != nil {
		return domain.Comment{}, domain.VideoSocialCounters{}, err
	}
	s.comments[comment.ID] = comment
	counter := s.counters[comment.VideoID]
	counter.CommentCount++
	counter.UpdatedAt = comment.CreatedAt
	s.counters[comment.VideoID] = counter
	return comment, counter, nil
}

func (s *MemoryStore) ListComments(_ context.Context, filter ListCommentsFilter) ([]domain.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureActiveFeedItemLocked(filter.VideoID); err != nil {
		return nil, err
	}
	limit := normalizedLimit(filter.Limit)
	comments := make([]domain.Comment, 0)
	for _, comment := range s.comments {
		if comment.VideoID != filter.VideoID || comment.Status != domain.CommentStatusVisible {
			continue
		}
		if filter.BeforeCreatedAt != nil && !commentBeforeCursor(comment, *filter.BeforeCreatedAt, filter.BeforeCommentID) {
			continue
		}
		comments = append(comments, comment)
	}
	sort.Slice(comments, func(i, j int) bool {
		if comments[i].CreatedAt.Equal(comments[j].CreatedAt) {
			return comments[i].ID > comments[j].ID
		}
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
	if len(comments) > limit {
		comments = comments[:limit]
	}
	return comments, nil
}

func (s *MemoryStore) DeleteComment(_ context.Context, commentID string, actorID string, actorRole string, now time.Time) (domain.Comment, domain.VideoSocialCounters, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	comment, ok := s.comments[commentID]
	if !ok {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, domain.NotFound(domain.CodeCommentNotFound, "Comment was not found.")
	}
	if comment.UserID != actorID && actorRole != "admin" {
		return domain.Comment{}, domain.VideoSocialCounters{}, false, domain.Forbidden("Only the comment owner can delete this comment.")
	}
	if comment.Status == domain.CommentStatusDeleted {
		return comment, s.counters[comment.VideoID], false, nil
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	comment.Status = domain.CommentStatusDeleted
	comment.Body = ""
	comment.UpdatedAt = now
	s.comments[comment.ID] = comment
	counter := s.counters[comment.VideoID]
	if counter.CommentCount > 0 {
		counter.CommentCount--
	}
	counter.UpdatedAt = now
	s.counters[comment.VideoID] = counter
	return comment, counter, true, nil
}

func (s *MemoryStore) SetFollow(_ context.Context, mutation FollowMutation, following bool) (domain.Follow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := mutation.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	key := followKey(mutation.FollowerID, mutation.FolloweeID)
	existing, exists := s.follows[key]
	changed := false
	if following {
		if !exists {
			existing = domain.Follow{
				ID:            domain.NewID("follow"),
				FollowerID:    strings.TrimSpace(mutation.FollowerID),
				FolloweeID:    strings.TrimSpace(mutation.FolloweeID),
				Status:        domain.FollowStatusActive,
				RequestID:     mutation.RequestID,
				CorrelationID: mutation.CorrelationID,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			changed = true
		} else if existing.Status != domain.FollowStatusActive {
			existing.Status = domain.FollowStatusActive
			existing.RequestID = mutation.RequestID
			existing.CorrelationID = mutation.CorrelationID
			existing.UpdatedAt = now
			changed = true
		}
		s.follows[key] = existing
		return existing, changed, nil
	}
	if exists && existing.Status == domain.FollowStatusActive {
		existing.Status = domain.FollowStatusDeleted
		existing.RequestID = mutation.RequestID
		existing.CorrelationID = mutation.CorrelationID
		existing.UpdatedAt = now
		s.follows[key] = existing
		changed = true
		return existing, changed, nil
	}
	if !exists {
		existing = domain.Follow{
			ID:         domain.NewID("follow"),
			FollowerID: strings.TrimSpace(mutation.FollowerID),
			FolloweeID: strings.TrimSpace(mutation.FolloweeID),
			Status:     domain.FollowStatusDeleted,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}
	return existing, false, nil
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

func (s *MemoryStore) ensureActiveFeedItemLocked(videoID string) error {
	item, ok := s.feedItems[strings.TrimSpace(videoID)]
	if !ok || item.Status != domain.FeedItemStatusActive {
		return domain.NotFound(domain.CodeFeedItemNotFound, "Feed item was not found.")
	}
	if _, ok := s.counters[item.VideoID]; !ok {
		s.counters[item.VideoID] = domain.VideoSocialCounters{VideoID: item.VideoID, UpdatedAt: item.UpdatedAt}
	}
	return nil
}

func likeKey(videoID string, userID string) string {
	return strings.TrimSpace(videoID) + ":" + strings.TrimSpace(userID)
}

func followKey(followerID string, followeeID string) string {
	return strings.TrimSpace(followerID) + ":" + strings.TrimSpace(followeeID)
}

func commentBeforeCursor(comment domain.Comment, beforeCreatedAt time.Time, beforeCommentID string) bool {
	beforeCreatedAt = beforeCreatedAt.UTC()
	if comment.CreatedAt.Before(beforeCreatedAt) {
		return true
	}
	if comment.CreatedAt.Equal(beforeCreatedAt) && beforeCommentID != "" {
		return comment.ID < beforeCommentID
	}
	return false
}
