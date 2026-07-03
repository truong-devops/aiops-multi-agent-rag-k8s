package service

import (
	"errors"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type RetryDecision struct {
	ErrorCode    string
	ErrorMessage string
	Retryable    bool
	RetryAt      *time.Time
	DeadLetter   bool
}

func DecideRetry(job domain.ProcessingJob, err error, now time.Time) RetryDecision {
	code := domain.CodeUnknownProcessingError
	message := err.Error()
	retryable := true
	var processingErr domain.ProcessingError
	if errors.As(err, &processingErr) {
		if processingErr.Code != "" {
			code = processingErr.Code
		}
		if processingErr.Message != "" {
			message = processingErr.Message
		}
		retryable = processingErr.Retryable
	} else if isAppCode(err, domain.CodeRawObjectNotFound) {
		code = domain.CodeRawObjectNotFound
		retryable = false
	} else if isAppCode(err, domain.CodeMinIOUnavailable) {
		code = domain.CodeMinIOUnavailable
		retryable = true
	}
	attemptNo := job.AttemptCount
	if attemptNo <= 0 {
		attemptNo = 1
	}
	if !retryable || attemptNo >= job.MaxAttempts {
		return RetryDecision{
			ErrorCode:    code,
			ErrorMessage: message,
			Retryable:    retryable,
			DeadLetter:   true,
		}
	}
	delay := backoffDelay(attemptNo)
	retryAt := now.UTC().Add(delay)
	return RetryDecision{
		ErrorCode:    code,
		ErrorMessage: message,
		Retryable:    true,
		RetryAt:      &retryAt,
	}
}

func backoffDelay(attemptNo int) time.Duration {
	if attemptNo <= 1 {
		return 10 * time.Second
	}
	delay := time.Duration(1<<(attemptNo-1)) * 10 * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func isAppCode(err error, code string) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Code == code
}
