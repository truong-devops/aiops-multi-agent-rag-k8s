package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/repository"
)

func TestCreateStartEndLiveSession(t *testing.T) {
	svc := newTestService()
	actor := Actor{UserID: "usr_creator", Role: "user"}

	created, err := svc.CreateSession(context.Background(), CreateSessionInput{
		Actor:         actor,
		Title:         "Demo live",
		Description:   "Kubernetes test",
		RequestID:     "req_test",
		CorrelationID: "corr_test",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if created.Session.Status != domain.LiveStatusScheduled {
		t.Fatalf("status = %q, want scheduled", created.Session.Status)
	}
	if created.StreamKey == "" {
		t.Fatal("stream key is empty")
	}
	if created.Session.StreamKeyHash == "" || created.Session.StreamKeyHash == created.StreamKey {
		t.Fatalf("stream key hash was not stored safely")
	}

	started, changed, err := svc.StartSession(context.Background(), created.Session.ID, actor, "req_start", "corr_test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	if !changed || started.Status != domain.LiveStatusLive || started.StartedAt == nil {
		t.Fatalf("start result changed=%v status=%q started_at=%v", changed, started.Status, started.StartedAt)
	}

	ended, changed, err := svc.EndSession(context.Background(), created.Session.ID, actor, "req_end", "corr_test")
	if err != nil {
		t.Fatalf("EndSession() error = %v", err)
	}
	if !changed || ended.Status != domain.LiveStatusEnded || ended.EndedAt == nil {
		t.Fatalf("end result changed=%v status=%q ended_at=%v", changed, ended.Status, ended.EndedAt)
	}
}

func TestStartEndAreIdempotentForCurrentState(t *testing.T) {
	svc := newTestService()
	actor := Actor{UserID: "usr_creator", Role: "user"}
	created, err := svc.CreateSession(context.Background(), CreateSessionInput{Actor: actor, Title: "Demo live"})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if _, _, err := svc.StartSession(context.Background(), created.Session.ID, actor, "req_start", "corr"); err != nil {
		t.Fatalf("StartSession() first error = %v", err)
	}
	again, changed, err := svc.StartSession(context.Background(), created.Session.ID, actor, "req_start_2", "corr")
	if err != nil {
		t.Fatalf("StartSession() second error = %v", err)
	}
	if changed || again.Status != domain.LiveStatusLive {
		t.Fatalf("second start changed=%v status=%q, want noop live", changed, again.Status)
	}
	if _, _, err := svc.EndSession(context.Background(), created.Session.ID, actor, "req_end", "corr"); err != nil {
		t.Fatalf("EndSession() first error = %v", err)
	}
	again, changed, err = svc.EndSession(context.Background(), created.Session.ID, actor, "req_end_2", "corr")
	if err != nil {
		t.Fatalf("EndSession() second error = %v", err)
	}
	if changed || again.Status != domain.LiveStatusEnded {
		t.Fatalf("second end changed=%v status=%q, want noop ended", changed, again.Status)
	}
}

func TestEndScheduledSessionReturnsInvalidState(t *testing.T) {
	svc := newTestService()
	actor := Actor{UserID: "usr_creator", Role: "user"}
	created, err := svc.CreateSession(context.Background(), CreateSessionInput{Actor: actor, Title: "Demo live"})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	_, _, err = svc.EndSession(context.Background(), created.Session.ID, actor, "req_end", "corr")
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("EndSession() error = %v, want AppError", err)
	}
	if appErr.Code != domain.CodeLiveInvalidState {
		t.Fatalf("error code = %q, want %q", appErr.Code, domain.CodeLiveInvalidState)
	}
}

func TestOwnerAuthorization(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateSession(context.Background(), CreateSessionInput{
		Actor: Actor{UserID: "usr_owner", Role: "user"},
		Title: "Owner live",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	_, _, err = svc.StartSession(context.Background(), created.Session.ID, Actor{UserID: "usr_other", Role: "user"}, "req", "corr")
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("StartSession() error = %v, want AppError", err)
	}
	if appErr.Code != domain.CodeForbidden {
		t.Fatalf("error code = %q, want FORBIDDEN", appErr.Code)
	}

	if _, _, err := svc.StartSession(context.Background(), created.Session.ID, Actor{UserID: "usr_admin", Role: "admin"}, "req", "corr"); err != nil {
		t.Fatalf("admin StartSession() error = %v", err)
	}
}

func TestListSessionsStatusAndCursor(t *testing.T) {
	svc := newTestService()
	actor := Actor{UserID: "usr_creator", Role: "user"}
	for _, title := range []string{"one", "two", "three"} {
		if _, err := svc.CreateSession(context.Background(), CreateSessionInput{Actor: actor, Title: title}); err != nil {
			t.Fatalf("CreateSession(%q) error = %v", title, err)
		}
	}
	page, err := svc.ListSessions(context.Background(), ListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(page.Sessions) != 2 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("first page len=%d has_more=%v cursor=%q", len(page.Sessions), page.HasMore, page.NextCursor)
	}
	next, err := svc.ListSessions(context.Background(), ListQuery{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("ListSessions(next) error = %v", err)
	}
	if len(next.Sessions) != 1 || next.HasMore {
		t.Fatalf("next page len=%d has_more=%v", len(next.Sessions), next.HasMore)
	}
}

func newTestService() *LiveService {
	svc := NewLiveService(repository.NewMemoryStore(), Options{
		DefaultLimit:    2,
		MaxLimit:        10,
		IngestBaseURL:   "rtmp://media.local/live",
		PlaybackBaseURL: "http://media.local/live",
		StreamKeyBytes:  24,
	})
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	return svc
}
