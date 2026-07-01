package event

import (
	"context"
	"log/slog"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type OutboxStore interface {
	ListPendingOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkOutboxFailed(ctx context.Context, id string, errMessage string) error
}

type OutboxMetrics interface {
	RecordOutboxPublish(outcome string)
	RecordDBOperation(operation string, outcome string, duration time.Duration)
}

type OutboxWorker struct {
	store        OutboxStore
	publisher    Publisher
	logger       *slog.Logger
	metrics      OutboxMetrics
	pollInterval time.Duration
	batchSize    int
	now          func() time.Time
}

type OutboxWorkerConfig struct {
	Store        OutboxStore
	Publisher    Publisher
	Logger       *slog.Logger
	Metrics      OutboxMetrics
	PollInterval time.Duration
	BatchSize    int
	Now          func() time.Time
}

func NewOutboxWorker(config OutboxWorkerConfig) *OutboxWorker {
	pollInterval := config.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 25
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &OutboxWorker{
		store:        config.Store,
		publisher:    config.Publisher,
		logger:       logger,
		metrics:      config.Metrics,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		now:          now,
	}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	if w == nil || w.store == nil || w.publisher == nil {
		return
	}
	w.logger.Info("outbox publisher started", "poll_interval", w.pollInterval.String(), "batch_size", w.batchSize)
	w.publishOnce(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox publisher stopped")
			return
		case <-ticker.C:
			w.publishOnce(ctx)
		}
	}
}

func (w *OutboxWorker) publishOnce(ctx context.Context) {
	startedAt := time.Now()
	events, err := w.store.ListPendingOutboxEvents(ctx, w.batchSize)
	w.recordDBOperation("list_outbox_events", startedAt, err)
	if err != nil {
		w.recordOutbox("list_error")
		w.logger.Error("failed to list outbox events", "error", err)
		return
	}
	if len(events) == 0 {
		return
	}
	w.recordOutbox("listed")

	for _, item := range events {
		if ctx.Err() != nil {
			return
		}
		if err := w.publisher.Publish(ctx, item); err != nil {
			w.recordOutbox("publish_error")
			w.logger.Error(
				"failed to publish outbox event",
				"event_id", item.ID,
				"event_name", item.EventName,
				"event_version", item.EventVersion,
				"aggregate_id", item.AggregateID,
				"attempts", item.Attempts+1,
				"request_id", item.RequestID,
				"correlation_id", item.CorrelationID,
				"error", err,
			)
			startedAt = time.Now()
			markErr := w.store.MarkOutboxFailed(ctx, item.ID, err.Error())
			w.recordDBOperation("mark_outbox_failed", startedAt, markErr)
			if markErr != nil {
				w.logger.Error("failed to mark outbox event failed", "event_id", item.ID, "error", markErr)
			}
			continue
		}
		publishedAt := w.now().UTC()
		startedAt = time.Now()
		if err := w.store.MarkOutboxPublished(ctx, item.ID, publishedAt); err != nil {
			w.recordDBOperation("mark_outbox_published", startedAt, err)
			w.recordOutbox("mark_published_error")
			w.logger.Error("failed to mark outbox event published", "event_id", item.ID, "error", err)
			continue
		}
		w.recordDBOperation("mark_outbox_published", startedAt, nil)
		w.recordOutbox("published")
		w.logger.Info(
			"outbox event published",
			"event_id", item.ID,
			"event_name", item.EventName,
			"event_version", item.EventVersion,
			"aggregate_id", item.AggregateID,
			"request_id", item.RequestID,
			"correlation_id", item.CorrelationID,
		)
	}
}

func (w *OutboxWorker) recordOutbox(outcome string) {
	if w.metrics != nil {
		w.metrics.RecordOutboxPublish(outcome)
	}
}

func (w *OutboxWorker) recordDBOperation(operation string, startedAt time.Time, err error) {
	if w.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	w.metrics.RecordDBOperation(operation, outcome, time.Since(startedAt))
}
