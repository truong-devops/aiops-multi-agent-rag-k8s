package repository

import (
	"context"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/observability"
)

type InstrumentedStore struct {
	next    Store
	metrics *observability.Metrics
}

func NewInstrumentedStore(next Store, metrics *observability.Metrics) *InstrumentedStore {
	return &InstrumentedStore{next: next, metrics: metrics}
}

func (s *InstrumentedStore) CreateSession(ctx context.Context, session domain.LiveSession, key domain.StreamKey, event domain.LiveEvent) error {
	startedAt := time.Now()
	err := s.next.CreateSession(ctx, session, key, event)
	s.record("create_session", err, startedAt)
	return err
}

func (s *InstrumentedStore) FindSessionByID(ctx context.Context, id string) (domain.LiveSession, error) {
	startedAt := time.Now()
	session, err := s.next.FindSessionByID(ctx, id)
	s.record("find_session", err, startedAt)
	return session, err
}

func (s *InstrumentedStore) ListSessions(ctx context.Context, filter ListSessionsFilter) ([]domain.LiveSession, error) {
	startedAt := time.Now()
	sessions, err := s.next.ListSessions(ctx, filter)
	s.record("list_sessions", err, startedAt)
	return sessions, err
}

func (s *InstrumentedStore) UpdateSessionState(ctx context.Context, session domain.LiveSession, event domain.LiveEvent) error {
	startedAt := time.Now()
	err := s.next.UpdateSessionState(ctx, session, event)
	s.record("update_session_state", err, startedAt)
	return err
}

func (s *InstrumentedStore) Ping(ctx context.Context) error {
	startedAt := time.Now()
	err := s.next.Ping(ctx)
	s.record("ping", err, startedAt)
	return err
}

func (s *InstrumentedStore) record(operation string, err error, startedAt time.Time) {
	if s.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	s.metrics.RecordDBOperation(operation, outcome, time.Since(startedAt))
}
