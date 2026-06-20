package domain

import "net/http"

const (
	CodeValidationError           = "VALIDATION_ERROR"
	CodeEmailAlreadyExists        = "EMAIL_ALREADY_EXISTS"
	CodeUsernameAlreadyExists     = "USERNAME_ALREADY_EXISTS"
	CodeWeakPassword              = "WEAK_PASSWORD"
	CodeInvalidCredentials        = "INVALID_CREDENTIALS"
	CodeUserDisabled              = "USER_DISABLED"
	CodeUnauthorized              = "UNAUTHORIZED"
	CodeRateLimited               = "RATE_LIMITED"
	CodeInvalidRefreshToken       = "INVALID_REFRESH_TOKEN"
	CodeRefreshTokenReused        = "REFRESH_TOKEN_REUSED"
	CodeSessionRevoked            = "SESSION_REVOKED"
	CodeGoogleNotConfigured       = "GOOGLE_NOT_CONFIGURED"
	CodeGoogleStateInvalid        = "GOOGLE_STATE_INVALID"
	CodeGoogleTokenExchangeFailed = "GOOGLE_TOKEN_EXCHANGE_FAILED"
	CodeGoogleIDTokenInvalid      = "GOOGLE_ID_TOKEN_INVALID"
	CodeGoogleEmailNotVerified    = "GOOGLE_EMAIL_NOT_VERIFIED"
	CodeRouteNotFound             = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed          = "METHOD_NOT_ALLOWED"
	CodeServiceNotReady           = "SERVICE_NOT_READY"
	CodeInternal                  = "INTERNAL_ERROR"
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
