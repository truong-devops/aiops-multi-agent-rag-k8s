package cache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

const feedKeySet = "feed:guest:keys"

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(redisURL string) (*RedisStore, error) {
	options, err := redis.ParseURL(strings.TrimSpace(redisURL))
	if err != nil {
		return nil, err
	}
	return &RedisStore{client: redis.NewClient(options)}, nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) GetFeed(ctx context.Context, key string) (FeedPage, bool, error) {
	raw, err := s.client.Get(ctx, feedKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return FeedPage{}, false, nil
	}
	if err != nil {
		return FeedPage{}, false, err
	}
	var page FeedPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return FeedPage{}, false, err
	}
	return page, true, nil
}

func (s *RedisStore) SetFeed(ctx context.Context, key string, page FeedPage, ttl time.Duration) error {
	raw, err := json.Marshal(page)
	if err != nil {
		return err
	}
	redisKey := feedKey(key)
	pipe := s.client.Pipeline()
	pipe.Set(ctx, redisKey, raw, ttl)
	pipe.SAdd(ctx, feedKeySet, redisKey)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) InvalidateFeed(ctx context.Context) error {
	keys, err := s.client.SMembers(ctx, feedKeySet).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keys...)
	pipe.Del(ctx, feedKeySet)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, bool, error) {
	raw, err := s.client.Get(ctx, countersKey(videoID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.VideoSocialCounters{}, false, nil
	}
	if err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	var counters domain.VideoSocialCounters
	if err := json.Unmarshal(raw, &counters); err != nil {
		return domain.VideoSocialCounters{}, false, err
	}
	return counters, true, nil
}

func (s *RedisStore) SetCounters(ctx context.Context, videoID string, counters domain.VideoSocialCounters, ttl time.Duration) error {
	raw, err := json.Marshal(counters)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, countersKey(videoID), raw, ttl).Err()
}

func (s *RedisStore) InvalidateCounters(ctx context.Context, videoID string) error {
	return s.client.Del(ctx, countersKey(videoID)).Err()
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func feedKey(key string) string {
	return "feed:guest:" + strings.TrimSpace(key)
}

func countersKey(videoID string) string {
	return "feed:video:" + strings.TrimSpace(videoID) + ":counters"
}
