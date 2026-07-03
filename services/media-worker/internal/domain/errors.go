package domain

import "net/http"

const (
	CodeValidationError  = "VALIDATION_ERROR"
	CodeJobNotFound      = "PROCESSING_JOB_NOT_FOUND"
	CodeAttemptNotFound  = "PROCESSING_ATTEMPT_NOT_FOUND"
	CodeInvalidJobState  = "INVALID_PROCESSING_JOB_STATE"
	CodeServiceNotReady  = "SERVICE_NOT_READY"
	CodeRouteNotFound    = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeInternal         = "INTERNAL_ERROR"
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
