package domain

import "time"

const (
	VideoStatusDraft      = "draft"
	VideoStatusUploaded   = "uploaded"
	VideoStatusProcessing = "processing"
	VideoStatusReady      = "ready"
	VideoStatusFailed     = "failed"
	VideoStatusDeleted    = "deleted"

	VisibilityPublic   = "public"
	VisibilityPrivate  = "private"
	VisibilityUnlisted = "unlisted"

	UploadStatusCreated   = "created"
	UploadStatusUploaded  = "uploaded"
	UploadStatusExpired   = "expired"
	UploadStatusCancelled = "cancelled"

	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)

type Video struct {
	ID                  string
	OwnerID             string
	Title               string
	Description         string
	Status              string
	Visibility          string
	RawObjectKey        string
	ProcessedObjectKey  string
	ThumbnailObjectKey  string
	ContentType         string
	SizeBytes           int64
	DurationMs          int64
	Width               int
	Height              int
	ProcessingErrorCode string
	PublishedAt         *time.Time
	DeletedAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastRequestID       string
	LastCorrelationID   string
}

type UploadRequest struct {
	ID             string
	VideoID        string
	OwnerID        string
	IdempotencyKey string
	Bucket         string
	ObjectKey      string
	Status         string
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	ExpiresAt      time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RequestID      string
	CorrelationID  string
}

type StatusHistory struct {
	ID             string
	VideoID        string
	PreviousStatus string
	NewStatus      string
	Reason         string
	ErrorCode      string
	RequestID      string
	CorrelationID  string
	CreatedAt      time.Time
}

type OutboxEvent struct {
	ID            string
	EventName     string
	EventVersion  string
	AggregateID   string
	Producer      string
	Environment   string
	Payload       []byte
	Status        string
	RequestID     string
	CorrelationID string
	OccurredAt    time.Time
	PublishedAt   *time.Time
	CreatedAt     time.Time
	Attempts      int
	LastError     string
}

func ValidVisibility(value string) bool {
	switch value {
	case VisibilityPublic, VisibilityPrivate, VisibilityUnlisted:
		return true
	default:
		return false
	}
}

func ValidVideoStatus(value string) bool {
	switch value {
	case VideoStatusDraft, VideoStatusUploaded, VideoStatusProcessing, VideoStatusReady, VideoStatusFailed, VideoStatusDeleted:
		return true
	default:
		return false
	}
}

func CanTransitionVideo(from string, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case VideoStatusDraft:
		return to == VideoStatusUploaded || to == VideoStatusDeleted
	case VideoStatusUploaded:
		return to == VideoStatusProcessing || to == VideoStatusFailed || to == VideoStatusDeleted
	case VideoStatusProcessing:
		return to == VideoStatusReady || to == VideoStatusFailed || to == VideoStatusDeleted
	case VideoStatusReady:
		return to == VideoStatusDeleted
	case VideoStatusFailed:
		return to == VideoStatusProcessing || to == VideoStatusDeleted
	default:
		return false
	}
}
