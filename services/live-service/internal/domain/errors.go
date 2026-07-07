package domain

import "net/http"

const (
	CodeValidationError     = "VALIDATION_ERROR"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeLiveSessionNotFound = "LIVE_SESSION_NOT_FOUND"
	CodeLiveInvalidState    = "LIVE_INVALID_STATE"
	CodeStreamKeyInvalid    = "STREAM_KEY_INVALID"
	CodeMediaMTXUnavailable = "MEDIAMTX_UNAVAILABLE"
	CodeServiceNotReady     = "SERVICE_NOT_READY"
	CodeRouteNotFound       = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed    = "METHOD_NOT_ALLOWED"
	CodeInternal            = "INTERNAL_ERROR"
)

type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string {
	return e.Code + ": " + e.Message
}

func NewError(status int, code string, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func ValidationError(message string) *AppError {
	return NewError(http.StatusBadRequest, CodeValidationError, message)
}

func Unauthorized(message string) *AppError {
	return NewError(http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return NewError(http.StatusForbidden, CodeForbidden, message)
}

func NotFound(code string, message string) *AppError {
	return NewError(http.StatusNotFound, code, message)
}

func Conflict(code string, message string) *AppError {
	return NewError(http.StatusConflict, code, message)
}
