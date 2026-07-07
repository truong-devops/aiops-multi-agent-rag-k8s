package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
)

type ListSessionsFilter struct {
	CreatorID       string
	Status          string
	Limit           int
	BeforeCreatedAt *time.Time
	BeforeSessionID string
}

type Store interface {
	CreateSession(ctx context.Context, session domain.LiveSession, key domain.StreamKey, event domain.LiveEvent) error
	FindSessionByID(ctx context.Context, id string) (domain.LiveSession, error)
	ListSessions(ctx context.Context, filter ListSessionsFilter) ([]domain.LiveSession, error)
	UpdateSessionState(ctx context.Context, session domain.LiveSession, event domain.LiveEvent) error
	Ping(ctx context.Context) error
}
