package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/live-service/internal/service"
)

type LiveService interface {
	Ready(ctx context.Context) error
	CreateSession(ctx context.Context, input service.CreateSessionInput) (service.CreateSessionResult, error)
	GetSession(ctx context.Context, id string, actor service.Actor) (domain.LiveSession, error)
	ListSessions(ctx context.Context, query service.ListQuery) (service.ListPage, error)
	StartSession(ctx context.Context, id string, actor service.Actor, requestID string, correlationID string) (domain.LiveSession, bool, error)
	EndSession(ctx context.Context, id string, actor service.Actor, requestID string, correlationID string) (domain.LiveSession, bool, error)
	IngestURL(session domain.LiveSession) string
	PlaybackURL(session domain.LiveSession) string
}

type Handler struct {
	service LiveService
}

func New(service LiveService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, metrics http.HandlerFunc) {
	mux.HandleFunc("/healthz", h.text("ok\n"))
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/v1/live-sessions", h.liveSessions)
	mux.HandleFunc("/v1/live-sessions/", h.liveSessionResource)
	if metrics != nil {
		mux.HandleFunc("/metrics", metrics)
	} else {
		mux.HandleFunc("/metrics", h.text("# metrics unavailable\n"))
	}
	mux.HandleFunc("/", h.notFound)
}

func (h *Handler) readyz(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	if h.service != nil {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := h.service.Ready(ctx); err != nil {
			writeError(w, req, domain.NewError(http.StatusServiceUnavailable, domain.CodeServiceNotReady, "Service is not ready."))
			return
		}
	}
	writeRawJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) liveSessions(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		h.createSession(w, req)
	case http.MethodGet:
		h.listSessions(w, req)
	default:
		writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
	}
}

func (h *Handler) createSession(w http.ResponseWriter, req *http.Request) {
	var body createLiveSessionRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, req, domain.ValidationError("request body must be valid JSON."))
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, req, domain.ValidationError("request body must contain a single JSON object."))
		return
	}
	input, err := body.toInput(req)
	if err != nil {
		writeError(w, req, err)
		return
	}
	result, err := h.service.CreateSession(req.Context(), input)
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusCreated, liveSessionEnvelope{
		Data:      h.sessionResponse(result.Session, result.StreamKey),
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) listSessions(w http.ResponseWriter, req *http.Request) {
	limit, err := parseOptionalInt(req.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, req, domain.ValidationError("limit must be an integer."))
		return
	}
	page, err := h.service.ListSessions(req.Context(), service.ListQuery{
		Actor:     actorFromRequest(req),
		Status:    req.URL.Query().Get("status"),
		CreatorID: req.URL.Query().Get("creator_id"),
		Limit:     limit,
		Cursor:    req.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	items := make([]liveSessionResponse, 0, len(page.Sessions))
	for _, session := range page.Sessions {
		items = append(items, h.sessionResponse(session, ""))
	}
	writeRawJSON(w, http.StatusOK, listEnvelope{
		Data:      items,
		Page:      pageResponse{Limit: page.Limit, NextCursor: page.NextCursor, HasMore: page.HasMore},
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) liveSessionResource(w http.ResponseWriter, req *http.Request) {
	sessionID, action, ok := parseLiveSessionResource(req.URL.Path)
	if !ok {
		h.notFound(w, req)
		return
	}
	switch action {
	case "":
		if !requireMethod(w, req, http.MethodGet) {
			return
		}
		session, err := h.service.GetSession(req.Context(), sessionID, actorFromRequest(req))
		if err != nil {
			writeError(w, req, err)
			return
		}
		writeRawJSON(w, http.StatusOK, liveSessionEnvelope{
			Data:      h.sessionResponse(session, ""),
			RequestID: observability.RequestIDFromContext(req.Context()),
		})
	case "start":
		h.startSession(w, req, sessionID)
	case "end":
		h.endSession(w, req, sessionID)
	default:
		h.notFound(w, req)
	}
}

func (h *Handler) startSession(w http.ResponseWriter, req *http.Request, sessionID string) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	session, changed, err := h.service.StartSession(req.Context(), sessionID, actorFromRequest(req), observability.RequestIDFromContext(req.Context()), observability.CorrelationIDFromContext(req.Context()))
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusAccepted, stateTransitionEnvelope{
		Data:      h.sessionResponse(session, ""),
		Changed:   changed,
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) endSession(w http.ResponseWriter, req *http.Request, sessionID string) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	session, changed, err := h.service.EndSession(req.Context(), sessionID, actorFromRequest(req), observability.RequestIDFromContext(req.Context()), observability.CorrelationIDFromContext(req.Context()))
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusAccepted, stateTransitionEnvelope{
		Data:      h.sessionResponse(session, ""),
		Changed:   changed,
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) sessionResponse(session domain.LiveSession, streamKey string) liveSessionResponse {
	return liveSessionResponse{
		ID:          session.ID,
		OwnerID:     session.CreatorID,
		CreatorID:   session.CreatorID,
		Title:       session.Title,
		Description: session.Description,
		Status:      session.Status,
		StreamKey:   streamKey,
		IngestURL:   h.service.IngestURL(session),
		PlaybackURL: h.service.PlaybackURL(session),
		ScheduledAt: session.ScheduledAt,
		StartedAt:   session.StartedAt,
		EndedAt:     session.EndedAt,
		FailureCode: emptyAsNil(session.FailureCode),
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
	}
}

func (h *Handler) text(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
}

func (h *Handler) notFound(w http.ResponseWriter, req *http.Request) {
	writeError(w, req, domain.NewError(http.StatusNotFound, domain.CodeRouteNotFound, "Route was not found."))
}

func requireMethod(w http.ResponseWriter, req *http.Request, method string) bool {
	if req.Method == method {
		return true
	}
	writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
	return false
}

func writeError(w http.ResponseWriter, req *http.Request, err error) {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		appErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternal, "Internal server error.")
	}
	writeRawJSON(w, appErr.Status, errorEnvelope{
		Error: errorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: map[string]string{},
		},
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func writeRawJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("extra JSON value")
}

func parseOptionalInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseLiveSessionResource(path string) (string, string, bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/live-sessions/"), "/")
	if trimmed == "" {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return parts[0], "", parts[0] != ""
	}
	if len(parts) == 2 {
		return parts[0], parts[1], parts[0] != "" && parts[1] != ""
	}
	return "", "", false
}

func actorFromRequest(req *http.Request) service.Actor {
	roles := strings.Split(req.Header.Get("X-User-Roles"), ",")
	role := ""
	for _, item := range roles {
		item = strings.TrimSpace(item)
		if item == "admin" {
			role = "admin"
			break
		}
		if role == "" && item != "" {
			role = item
		}
	}
	return service.Actor{
		UserID: strings.TrimSpace(req.Header.Get("X-User-ID")),
		Role:   role,
	}
}

func emptyAsNil(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

type createLiveSessionRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

func (r createLiveSessionRequest) toInput(req *http.Request) (service.CreateSessionInput, error) {
	return service.CreateSessionInput{
		Actor:         actorFromRequest(req),
		Title:         r.Title,
		Description:   r.Description,
		ScheduledAt:   r.ScheduledAt,
		RequestID:     observability.RequestIDFromContext(req.Context()),
		CorrelationID: observability.CorrelationIDFromContext(req.Context()),
	}, nil
}

type liveSessionEnvelope struct {
	Data      liveSessionResponse `json:"data"`
	RequestID string              `json:"request_id,omitempty"`
}

type stateTransitionEnvelope struct {
	Data      liveSessionResponse `json:"data"`
	Changed   bool                `json:"changed"`
	RequestID string              `json:"request_id,omitempty"`
}

type listEnvelope struct {
	Data      []liveSessionResponse `json:"data"`
	Page      pageResponse          `json:"page"`
	RequestID string                `json:"request_id,omitempty"`
}

type pageResponse struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type liveSessionResponse struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	CreatorID   string     `json:"creator_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StreamKey   string     `json:"stream_key,omitempty"`
	IngestURL   string     `json:"ingest_url"`
	PlaybackURL string     `json:"playback_url"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	FailureCode *string    `json:"failure_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type errorEnvelope struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}
