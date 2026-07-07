package domain

import "net/http"

const (
	CodeValidationError        = "VALIDATION_ERROR"
	CodeFeedItemNotFound       = "FEED_ITEM_NOT_FOUND"
	CodeInvalidFeedItemState   = "INVALID_FEED_ITEM_STATE"
	CodeServiceNotReady        = "SERVICE_NOT_READY"
	CodeUnauthorized           = "UNAUTHORIZED"
	CodeForbidden              = "FORBIDDEN"
	CodeInternalAPIUnavailable = "INTERNAL_API_UNAVAILABLE"
	CodeCommentNotFound        = "COMMENT_NOT_FOUND"
	CodeRouteNotFound          = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed       = "METHOD_NOT_ALLOWED"
	CodeInternal               = "INTERNAL_ERROR"
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

func NotFound(code string, message string) *AppError {
	return NewError(http.StatusNotFound, code, message)
}

func Conflict(code string, message string) *AppError {
	return NewError(http.StatusConflict, code, message)
}

func Forbidden(message string) *AppError {
	return NewError(http.StatusForbidden, CodeForbidden, message)
}
