package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	script *redis.Script
}

var allowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

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
	return &RedisLimiter{client: client, prefix: prefix, script: allowScript}, nil
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (Decision, error) {
	if limit <= 0 || window <= 0 {
		return Decision{Allowed: true, Limit: limit, Remaining: limit}, nil
	}
	fullKey := l.fullKey(key)
	windowMs := int64(window / time.Millisecond)
	if windowMs < 1 {
		windowMs = 1
	}
	result, err := l.script.Run(ctx, l.client, []string{fullKey}, windowMs).Slice()
	if err != nil {
		return Decision{}, err
	}
	if len(result) != 2 {
		return Decision{}, fmt.Errorf("unexpected redis rate limit result length %d", len(result))
	}
	count, err := redisInt64(result[0])
	if err != nil {
		return Decision{}, fmt.Errorf("decode redis rate limit count: %w", err)
	}
	ttlMs, err := redisInt64(result[1])
	if err != nil {
		return Decision{}, fmt.Errorf("decode redis rate limit ttl: %w", err)
	}
	if ttlMs < 0 {
		ttlMs = windowMs
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:    count <= limit,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: time.Duration(ttlMs) * time.Millisecond,
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

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}
