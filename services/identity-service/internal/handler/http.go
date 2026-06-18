package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/observability"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/identity-service/internal/service"
)

type Handler struct {
	auth   *service.AuthService
	google *service.GoogleOAuthService
}

func New(auth *service.AuthService, google *service.GoogleOAuthService) *Handler {
	return &Handler{auth: auth, google: google}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.text("ok\n"))
	mux.HandleFunc("/readyz", h.text("ready\n"))
	mux.HandleFunc("/metrics", h.text("# metrics placeholder\n"))
	mux.HandleFunc("/.well-known/jwks.json", h.jwks)
	mux.HandleFunc("/v1/auth/register", h.register)
	mux.HandleFunc("/v1/auth/login", h.login)
	mux.HandleFunc("/v1/auth/refresh", h.refresh)
	mux.HandleFunc("/v1/auth/logout", h.logout)
	mux.HandleFunc("/v1/auth/google/start", h.googleStart)
	mux.HandleFunc("/v1/auth/google/token", h.googleToken)
	mux.HandleFunc("/v1/users/me", h.currentUser)
	mux.HandleFunc("/", h.notFound)
}

func (h *Handler) register(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	user, err := h.auth.Register(req.Context(), service.RegisterInput{
		Email:       body.Email,
		Username:    body.Username,
		DisplayName: body.DisplayName,
		Password:    body.Password,
		IPAddress:   clientIP(req),
		UserAgent:   req.UserAgent(),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusCreated, map[string]any{"user": userResponse(user)})
}

func (h *Handler) login(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	result, err := h.auth.Login(req.Context(), service.LoginInput{
		Email:     body.Email,
		Password:  body.Password,
		IPAddress: clientIP(req),
		UserAgent: req.UserAgent(),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, authResultResponse(result))
}

func (h *Handler) refresh(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	result, err := h.auth.Refresh(req.Context(), body.RefreshToken)
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"token_type":    result.TokenType,
		"expires_in":    result.ExpiresIn,
	})
}

func (h *Handler) logout(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	if err := h.auth.Logout(req.Context(), body.RefreshToken); err != nil {
		writeError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) currentUser(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		user, _, err := h.auth.Authenticate(req.Context(), req.Header.Get("Authorization"))
		if err != nil {
			writeError(w, req, err)
			return
		}
		writeJSON(w, req, http.StatusOK, map[string]any{"user": userResponse(user)})
	case http.MethodPatch:
		user, _, err := h.auth.Authenticate(req.Context(), req.Header.Get("Authorization"))
		if err != nil {
			writeError(w, req, err)
			return
		}
		var body struct {
			DisplayName string `json:"display_name"`
			AvatarURL   string `json:"avatar_url"`
		}
		if !decodeJSON(w, req, &body) {
			return
		}
		updated, err := h.auth.UpdateProfile(req.Context(), user.ID, body.DisplayName, body.AvatarURL)
		if err != nil {
			writeError(w, req, err)
			return
		}
		writeJSON(w, req, http.StatusOK, map[string]any{"user": userResponse(updated)})
	default:
		methodNotAllowed(w, req)
	}
}

func (h *Handler) googleStart(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	result, err := h.google.Start(req.Context(), req.URL.Query().Get("redirect_uri"))
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, result)
}

func (h *Handler) googleToken(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodPost) {
		return
	}
	var body struct {
		Code         string `json:"code"`
		State        string `json:"state"`
		CodeVerifier string `json:"code_verifier"`
		RedirectURI  string `json:"redirect_uri"`
	}
	if !decodeJSON(w, req, &body) {
		return
	}
	identity, err := h.google.Exchange(req.Context(), service.GoogleTokenInput{
		Code:         body.Code,
		State:        body.State,
		CodeVerifier: body.CodeVerifier,
		RedirectURI:  body.RedirectURI,
		IPAddress:    clientIP(req),
		UserAgent:    req.UserAgent(),
	})
	if err != nil {
		writeError(w, req, err)
		return
	}
	user, err := h.google.UpsertIdentity(req.Context(), identity)
	if err != nil {
		writeError(w, req, err)
		return
	}
	result, err := h.auth.IssueForOAuthUser(req.Context(), user, req.UserAgent(), clientIP(req))
	if err != nil {
		writeError(w, req, err)
		return
	}
	writeJSON(w, req, http.StatusOK, authResultResponse(result))
}

func (h *Handler) jwks(w http.ResponseWriter, req *http.Request) {
	if !requireMethod(w, req, http.MethodGet) {
		return
	}
	writeRawJSON(w, http.StatusOK, h.auth.JWKS())
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
	return true
}

func writeJSON(w http.ResponseWriter, req *http.Request, status int, data any) {
	writeRawJSON(w, status, responseEnvelope{
		Data:      data,
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

func clientIP(req *http.Request) string {
	if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}
	return req.RemoteAddr
}

func authResultResponse(result service.AuthResult) map[string]any {
	return map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"token_type":    result.TokenType,
		"expires_in":    result.ExpiresIn,
		"user":          userResponse(result.User),
	}
}

func userResponse(user domain.User) map[string]any {
	return map[string]any{
		"id":             user.ID,
		"email":          user.Email,
		"username":       user.Username,
		"display_name":   user.DisplayName,
		"avatar_url":     user.AvatarURL,
		"status":         user.Status,
		"roles":          user.Roles,
		"email_verified": user.EmailVerified,
		"created_at":     user.CreatedAt.Format(time.RFC3339),
		"updated_at":     user.UpdatedAt.Format(time.RFC3339),
	}
}

type responseEnvelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id,omitempty"`
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
