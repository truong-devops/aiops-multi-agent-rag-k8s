package domain

import "net/http"

const (
	CodeValidationError         = "VALIDATION_ERROR"
	CodeJobNotFound             = "PROCESSING_JOB_NOT_FOUND"
	CodeAttemptNotFound         = "PROCESSING_ATTEMPT_NOT_FOUND"
	CodeInvalidJobState         = "INVALID_PROCESSING_JOB_STATE"
	CodeRawObjectNotFound       = "RAW_OBJECT_NOT_FOUND"
	CodeMinIOUnavailable        = "MINIO_UNAVAILABLE"
	CodeProcessTimeout          = "PROCESS_TIMEOUT"
	CodeFFmpegFailed            = "FFMPEG_FAILED"
	CodeVideoServiceUnavailable = "VIDEO_SERVICE_UNAVAILABLE"
	CodeUnknownProcessingError  = "UNKNOWN_PROCESSING_ERROR"
	CodeServiceNotReady         = "SERVICE_NOT_READY"
	CodeRouteNotFound           = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed        = "METHOD_NOT_ALLOWED"
	CodeInternal                = "INTERNAL_ERROR"
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

type ProcessingError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e ProcessingError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}
