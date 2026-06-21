package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNoopLimiterAlwaysAllows(t *testing.T) {
	decision, err := NoopLimiter{}.Allow(context.Background(), "rl:test", 5, time.Minute)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !decision.Allowed {
		t.Fatal("Allow().Allowed = false, want true")
	}
	if decision.Limit != 5 || decision.Remaining != 5 {
		t.Fatalf("Allow() = %+v, want limit and remaining 5", decision)
	}
}

func TestRedisInt64(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int64
	}{
		{name: "int64", value: int64(12), want: 12},
		{name: "int", value: 7, want: 7},
		{name: "string", value: "42", want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := redisInt64(tt.value)
			if err != nil {
				t.Fatalf("redisInt64() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("redisInt64() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRedisInt64RejectsUnexpectedType(t *testing.T) {
	if _, err := redisInt64(struct{}{}); err == nil {
		t.Fatal("redisInt64() error = nil, want error")
	}
}

func TestRedisInt64RejectsInvalidString(t *testing.T) {
	if _, err := redisInt64("42x"); err == nil {
		t.Fatal("redisInt64() error = nil, want invalid string error")
	}
}
