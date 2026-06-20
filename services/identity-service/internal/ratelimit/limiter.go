package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Decision struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (Decision, error)
}

type NoopLimiter struct{}

func (NoopLimiter) Allow(_ context.Context, _ string, limit int64, _ time.Duration) (Decision, error) {
	return Decision{Allowed: true, Limit: limit, Remaining: limit}, nil
}

type RedisLimiter struct {
	client *redis.Client
	prefix string
}

func NewRedisLimiter(ctx context.Context, redisURL string, prefix string) (*RedisLimiter, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, errors.New("redis url is required")
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(options)
	if ctx == nil {
		ctx = context.Background()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	if prefix == "" {
		prefix = "identity"
	}
	return &RedisLimiter{client: client, prefix: prefix}, nil
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (Decision, error) {
	if limit <= 0 || window <= 0 {
		return Decision{Allowed: true, Limit: limit, Remaining: limit}, nil
	}
	fullKey := l.fullKey(key)
	count, err := l.client.Incr(ctx, fullKey).Result()
	if err != nil {
		return Decision{}, err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, fullKey, window).Err(); err != nil {
			return Decision{}, err
		}
	}
	ttl, err := l.client.TTL(ctx, fullKey).Result()
	if err != nil {
		return Decision{}, err
	}
	if ttl < 0 {
		ttl = window
		_ = l.client.Expire(ctx, fullKey, window).Err()
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:    count <= limit,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: ttl,
	}, nil
}

func (l *RedisLimiter) Ping(ctx context.Context) error {
	return l.client.Ping(ctx).Err()
}

func (l *RedisLimiter) Close() error {
	return l.client.Close()
}

func (l *RedisLimiter) fullKey(key string) string {
	key = strings.Trim(strings.TrimSpace(key), ":")
	if key == "" {
		key = "unknown"
	}
	return l.prefix + ":" + key
}
