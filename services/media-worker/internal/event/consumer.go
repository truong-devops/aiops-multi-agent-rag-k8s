package event

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
)

type UploadedEventService interface {
	RegisterUploadedEvent(ctx context.Context, event domain.UploadedVideoEvent) (domain.ProcessingJob, bool, error)
}

type ConsumerMetrics interface {
	RecordJobOperation(operation string, outcome string)
	RecordEventAge(source string, outcome string, age time.Duration)
}

type Message struct {
	Key      []byte
	Value    []byte
	Time     time.Time
	kafka    kafka.Message
	hasKafka bool
}

type MessageConsumer interface {
	Fetch(ctx context.Context) (Message, error)
	Commit(ctx context.Context, message Message) error
	Close() error
}

type KafkaConsumer struct {
	reader *kafka.Reader
}

type KafkaConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewKafkaConsumer(config KafkaConsumerConfig) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  config.Brokers,
			Topic:    config.Topic,
			GroupID:  config.GroupID,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
	}
}

func (c *KafkaConsumer) Fetch(ctx context.Context) (Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}
	return Message{Key: msg.Key, Value: msg.Value, Time: msg.Time, kafka: msg, hasKafka: true}, nil
}

func (c *KafkaConsumer) Commit(ctx context.Context, message Message) error {
	if message.hasKafka {
		return c.reader.CommitMessages(ctx, message.kafka)
	}
	return c.reader.CommitMessages(ctx, kafka.Message{Key: message.Key, Value: message.Value, Time: message.Time})
}

func (c *KafkaConsumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

type UploadedConsumerWorker struct {
	consumer    MessageConsumer
	service     UploadedEventService
	logger      *slog.Logger
	metrics     ConsumerMetrics
	environment string
	now         func() time.Time
}

type UploadedConsumerConfig struct {
	Consumer    MessageConsumer
	Service     UploadedEventService
	Logger      *slog.Logger
	Metrics     ConsumerMetrics
	Environment string
	Now         func() time.Time
}

func NewUploadedConsumerWorker(config UploadedConsumerConfig) *UploadedConsumerWorker {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &UploadedConsumerWorker{
		consumer:    config.Consumer,
		service:     config.Service,
		logger:      logger,
		metrics:     config.Metrics,
		environment: defaultEnvironment(config.Environment),
		now:         now,
	}
}

func (w *UploadedConsumerWorker) Run(ctx context.Context) {
	if w == nil || w.consumer == nil || w.service == nil {
		return
	}
	w.logger.Info("uploaded event consumer started", "service", "media-worker", "environment", w.environment)
	for {
		if ctx.Err() != nil {
			w.logger.Info("uploaded event consumer stopped", "service", "media-worker", "environment", w.environment)
			return
		}
		message, err := w.consumer.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				w.logger.Info("uploaded event consumer stopped", "service", "media-worker", "environment", w.environment)
				return
			}
			w.record("consume", "fetch_error")
			w.logger.Error("failed to fetch uploaded event", "service", "media-worker", "environment", w.environment, "error", err)
			continue
		}
		if err := w.Handle(ctx, message); err != nil {
			w.logger.Error("failed to handle uploaded event", "service", "media-worker", "environment", w.environment, "error", err)
		}
	}
}

func (w *UploadedConsumerWorker) Handle(ctx context.Context, message Message) error {
	receivedAt := w.now().UTC()
	event, err := ParseUploadedEvent(message.Value, receivedAt)
	if err != nil {
		w.record("consume", "invalid")
		w.recordEventAge("invalid", receivedAt, message.Time)
		w.logger.Error("invalid uploaded event", "service", "media-worker", "environment", w.environment, "error", err)
		if commitErr := w.consumer.Commit(ctx, message); commitErr != nil {
			w.record("consume", "commit_error")
			return commitErr
		}
		return nil
	}
	job, created, err := w.service.RegisterUploadedEvent(ctx, event)
	if err != nil {
		w.record("consume", "handler_error")
		w.recordEventAge("handler_error", receivedAt, event.OccurredAt)
		return err
	}
	outcome := "created"
	if !created {
		outcome = "duplicate"
	}
	w.record("consume", outcome)
	w.recordEventAge(outcome, receivedAt, event.OccurredAt)
	if err := w.consumer.Commit(ctx, message); err != nil {
		w.record("consume", "commit_error")
		return err
	}
	w.logger.Info(
		"uploaded event consumed",
		"service", "media-worker",
		"environment", w.environment,
		"job_id", job.ID,
		"video_id", job.VideoID,
		"event_id", event.EventID,
		"outcome", outcome,
		"request_id", event.RequestID,
		"correlation_id", event.CorrelationID,
	)
	return nil
}

func (w *UploadedConsumerWorker) Close() error {
	if w == nil || w.consumer == nil {
		return nil
	}
	return w.consumer.Close()
}

func (w *UploadedConsumerWorker) record(operation string, outcome string) {
	if w.metrics != nil {
		w.metrics.RecordJobOperation(operation, outcome)
	}
}

func (w *UploadedConsumerWorker) recordEventAge(outcome string, receivedAt time.Time, occurredAt time.Time) {
	if w.metrics == nil || occurredAt.IsZero() {
		return
	}
	w.metrics.RecordEventAge("video.uploaded", outcome, receivedAt.Sub(occurredAt.UTC()))
}

func defaultEnvironment(value string) string {
	if value == "" {
		return "local"
	}
	return value
}
