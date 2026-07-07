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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/service"
)

type FeedService interface {
	Ready(ctx context.Context) error
	ListFeed(ctx context.Context, query service.FeedQuery) (service.FeedPage, error)
	GetSocialCounters(ctx context.Context, videoID string) (domain.VideoSocialCounters, error)
	LikeVideo(ctx context.Context, videoID string, actor service.Actor, requestID string, correlationID string) (domain.VideoSocialCounters, bool, error)
	UnlikeVideo(ctx context.Context, videoID string, actor service.Actor, requestID string, correlationID string) (domain.VideoSocialCounters, bool, error)
	CreateComment(ctx context.Context, input service.CreateCommentInput) (domain.Comment, domain.VideoSocialCounters, error)
	ListComments(ctx context.Context, query service.CommentQuery) (service.CommentPage, error)
	DeleteComment(ctx context.Context, commentID string, actor service.Actor) (domain.Comment, domain.VideoSocialCounters, bool, error)
	UpsertReadyVideo(ctx context.Context, input domain.ReadyVideoInput) (domain.FeedItem, bool, error)
}

type Handler struct {
	service          FeedService
	internalAPIToken string
}

type Options struct {
	InternalAPIToken string
}

func New(service FeedService, options ...Options) *Handler {
	var option Options
	if len(options) > 0 {
		option = options[0]
	}
	return &Handler{service: service, internalAPIToken: strings.TrimSpace(option.InternalAPIToken)}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, metrics http.HandlerFunc) {
	mux.HandleFunc("/healthz", h.text("ok\n"))
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/v1/feed", h.listFeed)
	mux.HandleFunc("/v1/videos/", h.videoResource)
	mux.HandleFunc("/v1/comments/", h.commentResource)
	mux.HandleFunc("/v1/internal/feed-items", h.ingestFeedItem)
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

func (h *Handler) listFeed(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	limit, err := parseOptionalInt(req.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, req, domain.ValidationError("limit must be an integer."))
		return
	}
	page, err := h.service.ListFeed(req.Context(), service.FeedQuery{
		Limit:  limit,
		Cursor: req.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusOK, feedEnvelope{
		Data:      feedItemsResponse(page.Items),
		Page:      pageResponse{Limit: page.Limit, NextCursor: page.NextCursor, HasMore: page.HasMore},
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) ingestFeedItem(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	if !h.authorizeInternal(w, req) {
		return
	}
	var body ingestReadyVideoRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, req, domain.ValidationError("request body must be valid JSON."))
		return
	}
	input := body.toInput(req)
	item, created, err := h.service.UpsertReadyVideo(req.Context(), input)
	if err != nil {
		writeError(w, req, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeRawJSON(w, status, readyVideoEnvelope{
		Data:      feedItemResponse(domain.FeedItemWithCounters{Item: item}),
		Created:   created,
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) videoResource(w http.ResponseWriter, req *http.Request) {
	videoID, resource, ok := parseNestedResource(req.URL.Path, "/v1/videos/")
	if !ok {
		h.notFound(w, req)
		return
	}
	switch resource {
	case "social":
		h.social(w, req, videoID)
	case "like":
		h.like(w, req, videoID)
	case "comments":
		h.videoComments(w, req, videoID)
	default:
		h.notFound(w, req)
	}
}

func (h *Handler) commentResource(w http.ResponseWriter, req *http.Request) {
	commentID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/v1/comments/"), "/")
	if commentID == "" || strings.Contains(commentID, "/") {
		h.notFound(w, req)
		return
	}
	if !requireMethod(w, req, http.MethodDelete) {
		return
	}
	comment, counters, deleted, err := h.service.DeleteComment(req.Context(), commentID, actorFromRequest(req))
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusOK, deleteCommentEnvelope{
		Data:      commentResponse(comment),
		Counters:  socialResponse(counters),
		Deleted:   deleted,
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) social(w http.ResponseWriter, req *http.Request, videoID string) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	counters, err := h.service.GetSocialCounters(req.Context(), videoID)
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusOK, socialEnvelope{
		Data:      socialResponse(counters),
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) like(w http.ResponseWriter, req *http.Request, videoID string) {
	actor := actorFromRequest(req)
	requestID := observability.RequestIDFromContext(req.Context())
	correlationID := observability.CorrelationIDFromContext(req.Context())
	var (
		counters domain.VideoSocialCounters
		changed  bool
		err      error
		liked    bool
	)
	switch req.Method {
	case http.MethodPut:
		counters, changed, err = h.service.LikeVideo(req.Context(), videoID, actor, requestID, correlationID)
		liked = true
	case http.MethodDelete:
		counters, changed, err = h.service.UnlikeVideo(req.Context(), videoID, actor, requestID, correlationID)
	default:
		writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
		return
	}
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusOK, likeEnvelope{
		Data:      socialResponse(counters),
		Liked:     liked,
		Changed:   changed,
		RequestID: requestID,
	})
}

func (h *Handler) videoComments(w http.ResponseWriter, req *http.Request, videoID string) {
	switch req.Method {
	case http.MethodGet:
		h.listComments(w, req, videoID)
	case http.MethodPost:
		h.createComment(w, req, videoID)
	default:
		writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
	}
}

func (h *Handler) listComments(w http.ResponseWriter, req *http.Request, videoID string) {
	limit, err := parseOptionalInt(req.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, req, domain.ValidationError("limit must be an integer."))
		return
	}
	page, err := h.service.ListComments(req.Context(), service.CommentQuery{
		VideoID: videoID,
		Limit:   limit,
		Cursor:  req.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusOK, commentsEnvelope{
		Data:      commentsResponse(page.Comments),
		Page:      pageResponse{Limit: page.Limit, NextCursor: page.NextCursor, HasMore: page.HasMore},
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) createComment(w http.ResponseWriter, req *http.Request, videoID string) {
	var body createCommentRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, req, domain.ValidationError("request body must be valid JSON."))
		return
	}
	comment, counters, err := h.service.CreateComment(req.Context(), service.CreateCommentInput{
		VideoID:       videoID,
		Actor:         actorFromRequest(req),
		Body:          body.Body,
		RequestID:     observability.RequestIDFromContext(req.Context()),
		CorrelationID: observability.CorrelationIDFromContext(req.Context()),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeRawJSON(w, http.StatusCreated, commentEnvelope{
		Data:      commentResponse(comment),
		Counters:  socialResponse(counters),
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func (h *Handler) notFound(w http.ResponseWriter, req *http.Request) {
	writeError(w, req, domain.NewError(http.StatusNotFound, domain.CodeRouteNotFound, "Route was not found."))
}

func (h *Handler) text(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, body)
	}
}

func writeRawJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, req *http.Request, err error) {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		appErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternal, "Unexpected server error.")
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

func requireMethod(w http.ResponseWriter, req *http.Request, method string) bool {
	if req.Method == method {
		return true
	}
	writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
	return false
}

func (h *Handler) authorizeInternal(w http.ResponseWriter, req *http.Request) bool {
	if h.internalAPIToken == "" {
		writeError(w, req, domain.NewError(http.StatusServiceUnavailable, domain.CodeInternalAPIUnavailable, "Internal ingestion API is not configured."))
		return false
	}
	if req.Header.Get("X-Internal-Token") != h.internalAPIToken {
		writeError(w, req, domain.NewError(http.StatusUnauthorized, domain.CodeUnauthorized, "Internal token is invalid."))
		return false
	}
	return true
}

func parseOptionalInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func feedItemsResponse(items []domain.FeedItemWithCounters) []feedItemBody {
	out := make([]feedItemBody, 0, len(items))
	for _, item := range items {
		out = append(out, feedItemResponse(item))
	}
	return out
}

func feedItemResponse(item domain.FeedItemWithCounters) feedItemBody {
	return feedItemBody{
		VideoID:            item.Item.VideoID,
		Owner:              ownerBody{ID: item.Item.OwnerID, DisplayName: ""},
		Title:              item.Item.Title,
		Description:        item.Item.Description,
		ThumbnailObjectKey: item.Item.ThumbnailObjectKey,
		PlaybackObjectKey:  item.Item.PlaybackObjectKey,
		DurationMs:         item.Item.DurationMs,
		LikeCount:          item.Counters.LikeCount,
		CommentCount:       item.Counters.CommentCount,
		ReadyAt:            item.Item.ReadyAt.UTC().Format(time.RFC3339Nano),
	}
}

func commentsResponse(comments []domain.Comment) []commentBody {
	out := make([]commentBody, 0, len(comments))
	for _, comment := range comments {
		out = append(out, commentResponse(comment))
	}
	return out
}

func commentResponse(comment domain.Comment) commentBody {
	return commentBody{
		ID:        comment.ID,
		VideoID:   comment.VideoID,
		UserID:    comment.UserID,
		Body:      comment.PublicBody(),
		Status:    comment.Status,
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: comment.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func socialResponse(counters domain.VideoSocialCounters) socialBody {
	return socialBody{
		VideoID:      counters.VideoID,
		LikeCount:    counters.LikeCount,
		CommentCount: counters.CommentCount,
		ShareCount:   counters.ShareCount,
	}
}

func actorFromRequest(req *http.Request) service.Actor {
	return service.Actor{
		UserID: strings.TrimSpace(req.Header.Get("X-User-ID")),
		Role:   actorRoleFromRequest(req),
	}
}

func actorRoleFromRequest(req *http.Request) string {
	if role := strings.ToLower(strings.TrimSpace(req.Header.Get("X-User-Role"))); role != "" {
		return role
	}
	for _, role := range strings.Split(req.Header.Get("X-User-Roles"), ",") {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "admin" {
			return role
		}
	}
	return ""
}

func parseNestedResource(path string, prefix string) (string, string, bool) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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

type feedEnvelope struct {
	Data      []feedItemBody `json:"data"`
	Page      pageResponse   `json:"page"`
	RequestID string         `json:"request_id,omitempty"`
}

type readyVideoEnvelope struct {
	Data      feedItemBody `json:"data"`
	Created   bool         `json:"created"`
	RequestID string       `json:"request_id,omitempty"`
}

type socialEnvelope struct {
	Data      socialBody `json:"data"`
	RequestID string     `json:"request_id,omitempty"`
}

type likeEnvelope struct {
	Data      socialBody `json:"data"`
	Liked     bool       `json:"liked"`
	Changed   bool       `json:"changed"`
	RequestID string     `json:"request_id,omitempty"`
}

type commentsEnvelope struct {
	Data      []commentBody `json:"data"`
	Page      pageResponse  `json:"page"`
	RequestID string        `json:"request_id,omitempty"`
}

type commentEnvelope struct {
	Data      commentBody `json:"data"`
	Counters  socialBody  `json:"counters"`
	RequestID string      `json:"request_id,omitempty"`
}

type deleteCommentEnvelope struct {
	Data      commentBody `json:"data"`
	Counters  socialBody  `json:"counters"`
	Deleted   bool        `json:"deleted"`
	RequestID string      `json:"request_id,omitempty"`
}

type feedItemBody struct {
	VideoID            string    `json:"video_id"`
	Owner              ownerBody `json:"owner"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	ThumbnailObjectKey string    `json:"thumbnail_object_key"`
	PlaybackObjectKey  string    `json:"playback_object_key"`
	DurationMs         int64     `json:"duration_ms"`
	LikeCount          int64     `json:"like_count"`
	CommentCount       int64     `json:"comment_count"`
	ReadyAt            string    `json:"ready_at"`
}

type socialBody struct {
	VideoID      string `json:"video_id"`
	LikeCount    int64  `json:"like_count"`
	CommentCount int64  `json:"comment_count"`
	ShareCount   int64  `json:"share_count"`
}

type commentBody struct {
	ID        string `json:"id"`
	VideoID   string `json:"video_id"`
	UserID    string `json:"user_id"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ownerBody struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type pageResponse struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type ingestReadyVideoRequest struct {
	EventID            string     `json:"event_id"`
	VideoID            string     `json:"video_id"`
	OwnerID            string     `json:"owner_id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	ThumbnailObjectKey string     `json:"thumbnail_object_key"`
	PlaybackObjectKey  string     `json:"playback_object_key"`
	DurationMs         int64      `json:"duration_ms"`
	Visibility         string     `json:"visibility"`
	ReadyAt            *time.Time `json:"ready_at"`
}

type createCommentRequest struct {
	Body string `json:"body"`
}

func (r ingestReadyVideoRequest) toInput(req *http.Request) domain.ReadyVideoInput {
	var readyAt time.Time
	if r.ReadyAt != nil {
		readyAt = r.ReadyAt.UTC()
	}
	return domain.ReadyVideoInput{
		EventID:            strings.TrimSpace(r.EventID),
		VideoID:            strings.TrimSpace(r.VideoID),
		OwnerID:            strings.TrimSpace(r.OwnerID),
		Title:              strings.TrimSpace(r.Title),
		Description:        strings.TrimSpace(r.Description),
		ThumbnailObjectKey: strings.TrimSpace(r.ThumbnailObjectKey),
		PlaybackObjectKey:  strings.TrimSpace(r.PlaybackObjectKey),
		DurationMs:         r.DurationMs,
		Visibility:         strings.TrimSpace(r.Visibility),
		RequestID:          observability.RequestIDFromContext(req.Context()),
		CorrelationID:      observability.CorrelationIDFromContext(req.Context()),
		ReadyAt:            readyAt,
	}
}
