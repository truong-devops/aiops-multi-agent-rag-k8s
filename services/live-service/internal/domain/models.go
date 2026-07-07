package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	LiveStatusScheduled = "scheduled"
	LiveStatusLive      = "live"
	LiveStatusEnded     = "ended"
	LiveStatusFailed    = "failed"
	LiveStatusCancelled = "cancelled"
)

const (
	StreamKeyStatusActive  = "active"
	StreamKeyStatusRotated = "rotated"
	StreamKeyStatusRevoked = "revoked"
)

const (
	LiveEventCreated = "created"
	LiveEventStarted = "started"
	LiveEventEnded   = "ended"
	LiveEventFailed  = "failed"
)

type LiveSession struct {
	ID                string
	CreatorID         string
	Title             string
	Description       string
	Status            string
	StreamKeyHash     string
	IngestPath        string
	PlaybackPath      string
	ScheduledAt       *time.Time
	StartedAt         *time.Time
	EndedAt           *time.Time
	FailureCode       string
	LastRequestID     string
	LastCorrelationID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type StreamKey struct {
	ID            string
	LiveSessionID string
	KeyHash       string
	Status        string
	CreatedAt     time.Time
	RotatedAt     *time.Time
	RevokedAt     *time.Time
}

type LiveEvent struct {
	ID            string
	LiveSessionID string
	EventType     string
	Payload       string
	RequestID     string
	CorrelationID string
	OccurredAt    time.Time
}

func NormalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func IsValidStatus(status string) bool {
	switch NormalizeStatus(status) {
	case LiveStatusScheduled, LiveStatusLive, LiveStatusEnded, LiveStatusFailed, LiveStatusCancelled:
		return true
	default:
		return false
	}
}

func CanStart(status string) bool {
	return NormalizeStatus(status) == LiveStatusScheduled
}

func CanEnd(status string) bool {
	return NormalizeStatus(status) == LiveStatusLive
}

func HashStreamKey(streamKey string) string {
	sum := sha256.Sum256([]byte(streamKey))
	return hex.EncodeToString(sum[:])
}
