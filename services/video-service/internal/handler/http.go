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

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/service"
)

type Handler struct {
	videos *service.VideoService
}

func New(videos *service.VideoService) *Handler {
	return &Handler{videos: videos}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, metrics http.HandlerFunc) {
	mux.HandleFunc("/healthz", h.text("ok\n"))
	mux.HandleFunc("/readyz", h.readyz)
	if metrics != nil {
		mux.HandleFunc("/metrics", metrics)
	} else {
		mux.HandleFunc("/metrics", h.text("# metrics unavailable\n"))
	}
	mux.HandleFunc("/v1/videos/upload-requests", h.createUploadRequest)
	mux.HandleFunc("/v1/videos/", h.videoByID)
	mux.HandleFunc("/v1/videos", h.videosCollection)
	mux.HandleFunc("/", h.notFound)
}

func (h *Handler) readyz(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	if err := h.videos.Ready(ctx); err != nil {
		writeError(w, req, domain.NewError(http.StatusServiceUnavailable, domain.CodeServiceNotReady, "Service is not ready."))
		return
	}
	writeRawJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) createUploadRequest(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		Title          string `json:"title"`
		Description    string `json:"description"`
		Visibility     string `json:"visibility"`
		ContentType    string `json:"content_type"`
		SizeBytes      int64  `json:"size_bytes"`
		ChecksumSHA256 string `json:"checksum_sha256"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	intent, err := h.videos.CreateUploadRequest(req.Context(), service.CreateUploadRequestInput{
		OwnerID:        userID(req),
		Title:          body.Title,
		Description:    body.Description,
		Visibility:     body.Visibility,
		ContentType:    body.ContentType,
		SizeBytes:      body.SizeBytes,
		ChecksumSHA256: body.ChecksumSHA256,
		RequestID:      observability.RequestIDFromContext(req.Context()),
		CorrelationID:  observability.CorrelationIDFromContext(req.Context()),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusCreated, map[string]any{
		"video":          videoResponse(intent.Video),
		"upload_request": uploadRequestResponse(intent.UploadRequest, intent.UploadURL),
	})
}

func (h *Handler) videosCollection(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		limit, err := parseLimit(req.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, req, err)
			return
		}
		videos, err := h.videos.ListVideos(req.Context(), repository.ListVideosFilter{
			OwnerID: strings.TrimSpace(req.URL.Query().Get("owner_id")),
			Status:  strings.TrimSpace(req.URL.Query().Get("status")),
			Limit:   limit,
		})
		if err != nil {
			writeError(w, req, err)
			return
		}
		items := make([]map[string]any, 0, len(videos))
		for _, video := range videos {
			items = append(items, videoResponse(video))
		}
		writeListJSON(w, req, items, limit)
	default:
		methodNotAllowed(w, req)
	}
}

func (h *Handler) videoByID(w http.ResponseWriter, req *http.Request) {
	trimmed := strings.TrimPrefix(req.URL.Path, "/v1/videos/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		h.notFound(w, req)
		return
	}
	videoID := parts[0]

	if len(parts) == 2 && parts[1] == "uploaded" {
		h.confirmUploaded(w, req, videoID)
		return
	}
	if len(parts) == 2 && parts[1] == "status" {
		h.updateStatus(w, req, videoID)
		return
	}
	if len(parts) != 1 {
		h.notFound(w, req)
		return
	}

	switch req.Method {
	case http.MethodGet:
		video, err := h.videos.GetVideo(req.Context(), videoID)
		if err != nil {
			writeError(w, req, err)
			return
		}
		writeJSON(w, req, http.StatusOK, map[string]any{"video": videoResponse(video)})
	default:
		methodNotAllowed(w, req)
	}
}

func (h *Handler) confirmUploaded(w http.ResponseWriter, req *http.Request, _ string) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		UploadRequestID string `json:"upload_request_id"`
		SizeBytes       int64  `json:"size_bytes"`
		ChecksumSHA256  string `json:"checksum_sha256"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	video, err := h.videos.ConfirmUploaded(req.Context(), service.ConfirmUploadedInput{
		UploadRequestID: body.UploadRequestID,
		SizeBytes:       body.SizeBytes,
		ChecksumSHA256:  body.ChecksumSHA256,
		RequestID:       observability.RequestIDFromContext(req.Context()),
		CorrelationID:   observability.CorrelationIDFromContext(req.Context()),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, map[string]any{"video": videoResponse(video)})
}

func (h *Handler) updateStatus(w http.ResponseWriter, req *http.Request, videoID string) {
	if !requireMethod(w, req, http.MethodPatch) {
		return
	}
	var body struct {
		Status    string `json:"status"`
		Reason    string `json:"reason"`
		ErrorCode string `json:"error_code"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	video, err := h.videos.UpdateStatus(req.Context(), service.UpdateStatusInput{
		VideoID:       videoID,
		Status:        body.Status,
		Reason:        body.Reason,
		ErrorCode:     body.ErrorCode,
		RequestID:     observability.RequestIDFromContext(req.Context()),
		CorrelationID: observability.CorrelationIDFromContext(req.Context()),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, map[string]any{"video": videoResponse(video)})
}

func (h *Handler) notFound(w http.ResponseWriter, req *http.Request) {
	writeError(w, req, domain.NewError(http.StatusNotFound, domain.CodeRouteNotFound, "Route was not found."))
}

func (h *Handler) text(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
}

func decodeJSON(w http.ResponseWriter, req *http.Request, out any) bool {
	if req.Body == nil {
		writeError(w, req, domain.ValidationError("Request body is required."))
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeError(w, req, domain.ValidationError("Request body must be valid JSON."))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, req, domain.ValidationError("Request body must contain a single JSON object."))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, req *http.Request, status int, data any) {
	writeRawJSON(w, status, responseEnvelope{
		Data:      data,
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
}

func writeListJSON(w http.ResponseWriter, req *http.Request, data any, limit int) {
	writeRawJSON(w, http.StatusOK, listEnvelope{
		Data: data,
		Page: pageBody{
			Limit:      limit,
			NextCursor: "",
			HasMore:    false,
		},
		RequestID: observability.RequestIDFromContext(req.Context()),
	})
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
	methodNotAllowed(w, req)
	return false
}

func methodNotAllowed(w http.ResponseWriter, req *http.Request) {
	writeError(w, req, domain.NewError(http.StatusMethodNotAllowed, domain.CodeMethodNotAllowed, "HTTP method is not allowed for this route."))
}

func userID(req *http.Request) string {
	return strings.TrimSpace(req.Header.Get("X-User-ID"))
}

func parseLimit(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 20, nil
	}
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit <= 0 {
		return 0, domain.ValidationError("limit must be a positive integer.")
	}
	if limit > 100 {
		limit = 100
	}
	return limit, nil
}

func videoResponse(video domain.Video) map[string]any {
	return map[string]any{
		"id":                    video.ID,
		"owner_id":              video.OwnerID,
		"title":                 video.Title,
		"description":           video.Description,
		"status":                video.Status,
		"visibility":            video.Visibility,
		"raw_object_key":        video.RawObjectKey,
		"processed_object_key":  video.ProcessedObjectKey,
		"thumbnail_object_key":  video.ThumbnailObjectKey,
		"content_type":          video.ContentType,
		"size_bytes":            video.SizeBytes,
		"duration_ms":           video.DurationMs,
		"width":                 video.Width,
		"height":                video.Height,
		"processing_error_code": video.ProcessingErrorCode,
		"published_at":          formatTimePtr(video.PublishedAt),
		"deleted_at":            formatTimePtr(video.DeletedAt),
		"created_at":            video.CreatedAt.Format(time.RFC3339),
		"updated_at":            video.UpdatedAt.Format(time.RFC3339),
	}
}

func uploadRequestResponse(upload domain.UploadRequest, uploadURL string) map[string]any {
	return map[string]any{
		"id":              upload.ID,
		"video_id":        upload.VideoID,
		"bucket":          upload.Bucket,
		"object_key":      upload.ObjectKey,
		"status":          upload.Status,
		"content_type":    upload.ContentType,
		"size_bytes":      upload.SizeBytes,
		"checksum_sha256": upload.ChecksumSHA256,
		"expires_at":      upload.ExpiresAt.Format(time.RFC3339),
		"upload_url":      uploadURL,
	}
}

func formatTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}

type responseEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}

type listEnvelope struct {
	Data      any      `json:"data"`
	Page      pageBody `json:"page"`
	RequestID string   `json:"request_id,omitempty"`
}

type pageBody struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
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
