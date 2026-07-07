package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/repository"
)

type Store = repository.Store

type Actor struct {
	UserID string
	Role   string
}

func (a Actor) IsAdmin() bool {
	return a.Role == "admin"
}

type Options struct {
	Metrics         *observability.Metrics
	Logger          *slog.Logger
	DefaultLimit    int
	MaxLimit        int
	IngestBaseURL   string
	PlaybackBaseURL string
	StreamKeyBytes  int
}

type LiveService struct {
	store           Store
	metrics         *observability.Metrics
	logger          *slog.Logger
	defaultLimit    int
	maxLimit        int
	ingestBaseURL   string
	playbackBaseURL string
	streamKeyBytes  int
	now             func() time.Time
}

type CreateSessionInput struct {
	Actor         Actor
	Title         string
	Description   string
	ScheduledAt   *time.Time
	RequestID     string
	CorrelationID string
}

type CreateSessionResult struct {
	Session   domain.LiveSession
	StreamKey string
}

type ListQuery struct {
	Actor     Actor
	Status    string
	CreatorID string
	Limit     int
	Cursor    string
}

type ListPage struct {
	Sessions   []domain.LiveSession
	Limit      int
	NextCursor string
	HasMore    bool
}

func NewLiveService(store Store, options Options) *LiveService {
	defaultLimit := options.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	maxLimit := options.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 50
	}
	if defaultLimit > maxLimit {
		defaultLimit = maxLimit
	}
	streamKeyBytes := options.StreamKeyBytes
	if streamKeyBytes <= 0 {
		streamKeyBytes = 32
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &LiveService{
		store:           store,
		metrics:         options.Metrics,
		logger:          logger,
		defaultLimit:    defaultLimit,
		maxLimit:        maxLimit,
		ingestBaseURL:   strings.TrimRight(options.IngestBaseURL, "/"),
		playbackBaseURL: strings.TrimRight(options.PlaybackBaseURL, "/"),
		streamKeyBytes:  streamKeyBytes,
		now:             func() time.Time { return time.Now().UTC() },
	}
}

func (s *LiveService) Ready(ctx context.Context) error {
	if s.store == nil {
		return domain.NewError(http.StatusServiceUnavailable, domain.CodeServiceNotReady, "Service store is not configured.")
	}
	return s.store.Ping(ctx)
}

func (s *LiveService) CreateSession(ctx context.Context, input CreateSessionInput) (CreateSessionResult, error) {
	if strings.TrimSpace(input.Actor.UserID) == "" {
		s.record("create_session", "unauthorized")
		return CreateSessionResult{}, domain.Unauthorized("Authentication is required to create a live session.")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		s.record("create_session", "validation_error")
		return CreateSessionResult{}, domain.ValidationError("title is required.")
	}
	if len(title) > 160 {
		s.record("create_session", "validation_error")
		return CreateSessionResult{}, domain.ValidationError("title must be at most 160 characters.")
	}
	description := strings.TrimSpace(input.Description)
	if len(description) > 2000 {
		s.record("create_session", "validation_error")
		return CreateSessionResult{}, domain.ValidationError("description must be at most 2000 characters.")
	}

	now := s.now().UTC()
	if input.ScheduledAt != nil {
		scheduled := input.ScheduledAt.UTC()
		input.ScheduledAt = &scheduled
	}
	streamKey, err := domain.NewSecret(s.streamKeyBytes)
	if err != nil {
		s.record("create_session", "error")
		return CreateSessionResult{}, err
	}
	sessionID := domain.NewID("live")
	ingestPath := sessionID
	session := domain.LiveSession{
		ID:                sessionID,
		CreatorID:         strings.TrimSpace(input.Actor.UserID),
		Title:             title,
		Description:       description,
		Status:            domain.LiveStatusScheduled,
		StreamKeyHash:     domain.HashStreamKey(streamKey),
		IngestPath:        ingestPath,
		PlaybackPath:      sessionID + "/index.m3u8",
		ScheduledAt:       input.ScheduledAt,
		LastRequestID:     input.RequestID,
		LastCorrelationID: input.CorrelationID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	key := domain.StreamKey{
		ID:            domain.NewID("sk"),
		LiveSessionID: session.ID,
		KeyHash:       session.StreamKeyHash,
		Status:        domain.StreamKeyStatusActive,
		CreatedAt:     now,
	}
	event := s.event(session, domain.LiveEventCreated, input.RequestID, input.CorrelationID, now)
	if err := s.store.CreateSession(ctx, session, key, event); err != nil {
		s.record("create_session", "error")
		return CreateSessionResult{}, err
	}
	s.record("create_session", "success")
	s.logger.Info("live session created", "service", "live-service", "live_session_id", session.ID, "creator_id", session.CreatorID, "request_id", input.RequestID, "correlation_id", input.CorrelationID)
	return CreateSessionResult{Session: session, StreamKey: streamKey}, nil
}

func (s *LiveService) GetSession(ctx context.Context, id string, actor Actor) (domain.LiveSession, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.LiveSession{}, domain.ValidationError("live_session_id is required.")
	}
	session, err := s.store.FindSessionByID(ctx, id)
	if err != nil {
		s.record("get_session", "error")
		return domain.LiveSession{}, err
	}
	s.record("get_session", "success")
	return session, nil
}

func (s *LiveService) ListSessions(ctx context.Context, query ListQuery) (ListPage, error) {
	limit := s.normalizeLimit(query.Limit)
	status := domain.NormalizeStatus(query.Status)
	if status != "" && !domain.IsValidStatus(status) {
		return ListPage{}, domain.ValidationError("status is invalid.")
	}
	beforeCreatedAt, beforeSessionID, err := decodeCursor(query.Cursor)
	if err != nil {
		return ListPage{}, domain.ValidationError("cursor is invalid.")
	}
	rows, err := s.store.ListSessions(ctx, repository.ListSessionsFilter{
		CreatorID:       strings.TrimSpace(query.CreatorID),
		Status:          status,
		Limit:           limit + 1,
		BeforeCreatedAt: beforeCreatedAt,
		BeforeSessionID: beforeSessionID,
	})
	if err != nil {
		s.record("list_sessions", "error")
		return ListPage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	s.record("list_sessions", "success")
	return ListPage{Sessions: rows, Limit: limit, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *LiveService) StartSession(ctx context.Context, id string, actor Actor, requestID string, correlationID string) (domain.LiveSession, bool, error) {
	session, err := s.loadOwnedSession(ctx, id, actor)
	if err != nil {
		s.record("start_session", "error")
		return domain.LiveSession{}, false, err
	}
	if session.Status == domain.LiveStatusLive {
		s.record("start_session", "noop")
		return session, false, nil
	}
	if !domain.CanStart(session.Status) {
		s.record("start_session", "invalid_state")
		return domain.LiveSession{}, false, domain.Conflict(domain.CodeLiveInvalidState, "Live session cannot be started from its current state.")
	}
	now := s.now().UTC()
	session.Status = domain.LiveStatusLive
	session.StartedAt = &now
	session.LastRequestID = requestID
	session.LastCorrelationID = correlationID
	session.UpdatedAt = now
	event := s.event(session, domain.LiveEventStarted, requestID, correlationID, now)
	if err := s.store.UpdateSessionState(ctx, session, event); err != nil {
		s.record("start_session", "error")
		return domain.LiveSession{}, false, err
	}
	s.record("start_session", "success")
	s.logger.Info("live session started", "service", "live-service", "live_session_id", session.ID, "creator_id", session.CreatorID, "request_id", requestID, "correlation_id", correlationID)
	return session, true, nil
}

func (s *LiveService) EndSession(ctx context.Context, id string, actor Actor, requestID string, correlationID string) (domain.LiveSession, bool, error) {
	session, err := s.loadOwnedSession(ctx, id, actor)
	if err != nil {
		s.record("end_session", "error")
		return domain.LiveSession{}, false, err
	}
	if session.Status == domain.LiveStatusEnded {
		s.record("end_session", "noop")
		return session, false, nil
	}
	if !domain.CanEnd(session.Status) {
		s.record("end_session", "invalid_state")
		return domain.LiveSession{}, false, domain.Conflict(domain.CodeLiveInvalidState, "Live session cannot be ended from its current state.")
	}
	now := s.now().UTC()
	session.Status = domain.LiveStatusEnded
	session.EndedAt = &now
	session.LastRequestID = requestID
	session.LastCorrelationID = correlationID
	session.UpdatedAt = now
	event := s.event(session, domain.LiveEventEnded, requestID, correlationID, now)
	if err := s.store.UpdateSessionState(ctx, session, event); err != nil {
		s.record("end_session", "error")
		return domain.LiveSession{}, false, err
	}
	s.record("end_session", "success")
	s.logger.Info("live session ended", "service", "live-service", "live_session_id", session.ID, "creator_id", session.CreatorID, "request_id", requestID, "correlation_id", correlationID)
	return session, true, nil
}

func (s *LiveService) IngestURL(session domain.LiveSession) string {
	return s.ingestBaseURL + "/" + session.IngestPath
}

func (s *LiveService) PlaybackURL(session domain.LiveSession) string {
	return s.playbackBaseURL + "/" + session.PlaybackPath
}

func (s *LiveService) loadOwnedSession(ctx context.Context, id string, actor Actor) (domain.LiveSession, error) {
	if strings.TrimSpace(actor.UserID) == "" {
		return domain.LiveSession{}, domain.Unauthorized("Authentication is required for this action.")
	}
	session, err := s.store.FindSessionByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return domain.LiveSession{}, err
	}
	if session.CreatorID != actor.UserID && !actor.IsAdmin() {
		return domain.LiveSession{}, domain.Forbidden("Only the session owner can perform this action.")
	}
	return session, nil
}

func (s *LiveService) event(session domain.LiveSession, eventType string, requestID string, correlationID string, occurredAt time.Time) domain.LiveEvent {
	payload := map[string]string{
		"live_session_id": session.ID,
		"creator_id":      session.CreatorID,
		"status":          session.Status,
	}
	encoded, _ := json.Marshal(payload)
	return domain.LiveEvent{
		ID:            domain.NewID("levt"),
		LiveSessionID: session.ID,
		EventType:     eventType,
		Payload:       string(encoded),
		RequestID:     requestID,
		CorrelationID: correlationID,
		OccurredAt:    occurredAt,
	}
}

func (s *LiveService) normalizeLimit(limit int) int {
	if limit <= 0 {
		return s.defaultLimit
	}
	if limit > s.maxLimit {
		return s.maxLimit
	}
	return limit
}

func (s *LiveService) record(operation string, outcome string) {
	if s.metrics != nil {
		s.metrics.RecordLiveOperation(operation, outcome)
	}
}

func encodeCursor(createdAt time.Time, id string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (*time.Time, string, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, "", nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, "", err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, "", strconv.ErrSyntax
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", err
	}
	createdAt = createdAt.UTC()
	return &createdAt, parts[1], nil
}
