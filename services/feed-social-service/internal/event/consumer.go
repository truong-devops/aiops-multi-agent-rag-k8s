package event

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/feed-social-service/internal/domain"
)

type ReadyVideoService interface {
	UpsertReadyVideo(ctx context.Context, input domain.ReadyVideoInput) (domain.FeedItem, bool, error)
}

type ConsumerMetrics interface {
	RecordFeedOperation(operation string, outcome string)
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

type ReadyConsumerWorker struct {
	consumer    MessageConsumer
	service     ReadyVideoService
	logger      *slog.Logger
	metrics     ConsumerMetrics
	environment string
	now         func() time.Time
}

type ReadyConsumerConfig struct {
	Consumer    MessageConsumer
	Service     ReadyVideoService
	Logger      *slog.Logger
	Metrics     ConsumerMetrics
	Environment string
	Now         func() time.Time
}

func NewReadyConsumerWorker(config ReadyConsumerConfig) *ReadyConsumerWorker {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &ReadyConsumerWorker{
		consumer:    config.Consumer,
		service:     config.Service,
		logger:      logger,
		metrics:     config.Metrics,
		environment: defaultEnvironment(config.Environment),
		now:         now,
	}
}

func (w *ReadyConsumerWorker) Run(ctx context.Context) {
	if w == nil || w.consumer == nil || w.service == nil {
		return
	}
	w.logger.Info("ready event consumer started", "service", "feed-social-service", "environment", w.environment)
	for {
		if ctx.Err() != nil {
			w.logger.Info("ready event consumer stopped", "service", "feed-social-service", "environment", w.environment)
			return
		}
		message, err := w.consumer.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				w.logger.Info("ready event consumer stopped", "service", "feed-social-service", "environment", w.environment)
				return
			}
			w.record("consume_ready_event", "fetch_error")
			w.logger.Error("failed to fetch ready event", "service", "feed-social-service", "environment", w.environment, "error", err)
			continue
		}
		if err := w.Handle(ctx, message); err != nil {
			w.logger.Error("failed to handle ready event", "service", "feed-social-service", "environment", w.environment, "error", err)
		}
	}
}

func (w *ReadyConsumerWorker) Handle(ctx context.Context, message Message) error {
	receivedAt := w.now().UTC()
	input, occurredAt, err := ParseReadyEvent(message.Value, receivedAt)
	if err != nil {
		w.record("consume_ready_event", "invalid")
		w.recordEventAge("invalid", receivedAt, message.Time)
		w.logger.Error("invalid ready event", "service", "feed-social-service", "environment", w.environment, "error", err)
		if commitErr := w.consumer.Commit(ctx, message); commitErr != nil {
			w.record("consume_ready_event", "commit_error")
			return commitErr
		}
		return nil
	}
	item, created, err := w.service.UpsertReadyVideo(ctx, input)
	if err != nil {
		w.record("consume_ready_event", "handler_error")
		w.recordEventAge("handler_error", receivedAt, occurredAt)
		return err
	}
	outcome := "created"
	if !created {
		outcome = "duplicate"
	}
	w.record("consume_ready_event", outcome)
	w.recordEventAge(outcome, receivedAt, occurredAt)
	if err := w.consumer.Commit(ctx, message); err != nil {
		w.record("consume_ready_event", "commit_error")
		return err
	}
	w.logger.Info(
		"ready event consumed",
		"service", "feed-social-service",
		"environment", w.environment,
		"video_id", item.VideoID,
		"owner_id", item.OwnerID,
		"event_id", input.EventID,
		"outcome", outcome,
		"request_id", input.RequestID,
		"correlation_id", input.CorrelationID,
	)
	return nil
}

func (w *ReadyConsumerWorker) Close() error {
	if w == nil || w.consumer == nil {
		return nil
	}
	return w.consumer.Close()
}

func (w *ReadyConsumerWorker) record(operation string, outcome string) {
	if w.metrics != nil {
		w.metrics.RecordFeedOperation(operation, outcome)
	}
}

func (w *ReadyConsumerWorker) recordEventAge(outcome string, receivedAt time.Time, occurredAt time.Time) {
	if w.metrics == nil || occurredAt.IsZero() {
		return
	}
	w.metrics.RecordEventAge("video.ready", outcome, receivedAt.Sub(occurredAt.UTC()))
}

func defaultEnvironment(value string) string {
	if value == "" {
		return "local"
	}
	return value
}
