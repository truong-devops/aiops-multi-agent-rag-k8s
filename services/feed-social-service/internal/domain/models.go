package domain

import (
	"strings"
	"time"
)

const (
	FeedItemStatusActive  = "active"
	FeedItemStatusHidden  = "hidden"
	FeedItemStatusDeleted = "deleted"

	InboxStatusProcessed = "processed"
	InboxStatusDuplicate = "duplicate"
)

type FeedItem struct {
	ID                 string
	VideoID            string
	OwnerID            string
	Title              string
	Description        string
	ThumbnailObjectKey string
	PlaybackObjectKey  string
	DurationMs         int64
	Visibility         string
	Status             string
	ReadyAt            time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type VideoSocialCounters struct {
	VideoID      string
	LikeCount    int64
	CommentCount int64
	ShareCount   int64
	UpdatedAt    time.Time
}

type FeedItemWithCounters struct {
	Item     FeedItem
	Counters VideoSocialCounters
}

type InboxEvent struct {
	ID            string
	EventName     string
	EventVersion  string
	AggregateID   string
	Status        string
	RequestID     string
	CorrelationID string
	ReceivedAt    time.Time
	ProcessedAt   *time.Time
}

type ReadyVideoInput struct {
	EventID            string
	VideoID            string
	OwnerID            string
	Title              string
	Description        string
	ThumbnailObjectKey string
	PlaybackObjectKey  string
	DurationMs         int64
	Visibility         string
	RequestID          string
	CorrelationID      string
	ReadyAt            time.Time
	ReceivedAt         time.Time
}

func NewFeedItemFromReadyVideo(input ReadyVideoInput, now time.Time) (FeedItem, error) {
	if strings.TrimSpace(input.VideoID) == "" {
		return FeedItem{}, ValidationError("video_id is required.")
	}
	if strings.TrimSpace(input.OwnerID) == "" {
		return FeedItem{}, ValidationError("owner_id is required.")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	readyAt := input.ReadyAt.UTC()
	if readyAt.IsZero() {
		readyAt = now
	}
	visibility := strings.TrimSpace(input.Visibility)
	if visibility == "" {
		visibility = "public"
	}
	status := FeedItemStatusActive
	if visibility != "public" {
		status = FeedItemStatusHidden
	}
	return FeedItem{
		ID:                 "feed_" + strings.TrimSpace(input.VideoID),
		VideoID:            strings.TrimSpace(input.VideoID),
		OwnerID:            strings.TrimSpace(input.OwnerID),
		Title:              strings.TrimSpace(input.Title),
		Description:        strings.TrimSpace(input.Description),
		ThumbnailObjectKey: strings.TrimSpace(input.ThumbnailObjectKey),
		PlaybackObjectKey:  strings.TrimSpace(input.PlaybackObjectKey),
		DurationMs:         input.DurationMs,
		Visibility:         visibility,
		Status:             status,
		ReadyAt:            readyAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func ValidFeedItemStatus(value string) bool {
	switch value {
	case FeedItemStatusActive, FeedItemStatusHidden, FeedItemStatusDeleted:
		return true
	default:
		return false
	}
}

func CanTransitionFeedItem(from string, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case FeedItemStatusActive:
		return to == FeedItemStatusHidden || to == FeedItemStatusDeleted
	case FeedItemStatusHidden:
		return to == FeedItemStatusActive || to == FeedItemStatusDeleted
	default:
		return false
	}
}
