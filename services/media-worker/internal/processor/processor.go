package processor

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/storage"
)

type Processor interface {
	Process(ctx context.Context, job domain.ProcessingJob, attempt domain.ProcessingAttempt) (Result, error)
}

type Result struct {
	ProcessedObjectKey string
	ThumbnailObjectKey string
	DurationMs         int64
	Width              int
	Height             int
	SizeBytes          int64
	Metrics            []byte
}

type PlaceholderProcessor struct {
	objectStore storage.ObjectStore
	failVideos  map[string]string
	now         func() time.Time
}

type PlaceholderConfig struct {
	ObjectStore storage.ObjectStore
	FailVideos  []string
	Now         func() time.Time
}

func NewPlaceholderProcessor(config PlaceholderConfig) *PlaceholderProcessor {
	failVideos := map[string]string{}
	for _, item := range config.FailVideos {
		parts := strings.SplitN(item, ":", 2)
		videoID := strings.TrimSpace(parts[0])
		if videoID == "" {
			continue
		}
		code := domain.CodeFFmpegFailed
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			code = strings.TrimSpace(parts[1])
		}
		failVideos[videoID] = code
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	objectStore := config.ObjectStore
	if objectStore == nil {
		objectStore = storage.NoopObjectStore{}
	}
	return &PlaceholderProcessor{
		objectStore: objectStore,
		failVideos:  failVideos,
		now:         now,
	}
}

func (p *PlaceholderProcessor) Process(ctx context.Context, job domain.ProcessingJob, attempt domain.ProcessingAttempt) (Result, error) {
	if code, ok := p.failVideos[job.VideoID]; ok {
		return Result{}, domain.ProcessingError{
			Code:      code,
			Message:   "placeholder processor forced failure",
			Retryable: code != domain.CodeRawObjectNotFound,
		}
	}
	metadata, err := p.objectStore.VerifyObject(ctx, storage.VerifyObjectInput{
		Bucket:    job.InputBucket,
		ObjectKey: job.InputObjectKey,
	})
	if err != nil {
		return Result{}, err
	}
	processedKey := "processed/" + job.VideoID + "/source.mp4"
	thumbnailKey := "thumbnails/" + job.VideoID + "/poster.jpg"
	metrics, _ := json.Marshal(map[string]any{
		"processor":    "placeholder",
		"attempt_no":   attempt.AttemptNo,
		"input_bytes":  metadata.SizeBytes,
		"generated_at": p.now().UTC().Format(time.RFC3339Nano),
	})
	return Result{
		ProcessedObjectKey: processedKey,
		ThumbnailObjectKey: thumbnailKey,
		DurationMs:         0,
		Width:              0,
		Height:             0,
		SizeBytes:          metadata.SizeBytes,
		Metrics:            metrics,
	}, nil
}
