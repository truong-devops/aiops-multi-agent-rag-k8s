package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
)

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
	Close() error
}

type KafkaPublisher struct {
	writer *kafka.Writer
	topic  string
}

type KafkaPublisherConfig struct {
	Brokers []string
	Topic   string
}

type Envelope struct {
	EventID       string          `json:"event_id"`
	EventName     string          `json:"event_name"`
	EventVersion  string          `json:"event_version"`
	EventType     string          `json:"event_type"`
	AggregateID   string          `json:"aggregate_id"`
	Producer      string          `json:"producer"`
	Environment   string          `json:"environment"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

func NewKafkaPublisher(config KafkaPublisherConfig) (*KafkaPublisher, error) {
	brokers := compactStrings(config.Brokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are required")
	}
	topic := strings.TrimSpace(config.Topic)
	if topic == "" {
		return nil, fmt.Errorf("kafka topic is required")
	}
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			RequiredAcks: kafka.RequireOne,
			Balancer:     &kafka.Hash{},
			BatchTimeout: 50 * time.Millisecond,
		},
		topic: topic,
	}, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, event domain.OutboxEvent) error {
	envelope, err := NewEnvelope(event)
	if err != nil {
		return err
	}
	value, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal outbox envelope %s: %w", event.ID, err)
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID),
		Value: value,
		Time:  event.OccurredAt,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.ID)},
			{Key: "event_name", Value: []byte(event.EventName)},
			{Key: "event_version", Value: []byte(event.EventVersion)},
			{Key: "producer", Value: []byte(event.Producer)},
			{Key: "correlation_id", Value: []byte(event.CorrelationID)},
		},
	})
}

func (p *KafkaPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func NewEnvelope(event domain.OutboxEvent) (Envelope, error) {
	if !json.Valid(event.Payload) {
		return Envelope{}, fmt.Errorf("outbox event %s payload is not valid json", event.ID)
	}
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = event.CreatedAt.UTC()
	}
	eventType := event.EventName
	if event.EventVersion != "" {
		eventType = event.EventName + "." + event.EventVersion
	}
	return Envelope{
		EventID:       event.ID,
		EventName:     event.EventName,
		EventVersion:  event.EventVersion,
		EventType:     eventType,
		AggregateID:   event.AggregateID,
		Producer:      event.Producer,
		Environment:   event.Environment,
		CorrelationID: event.CorrelationID,
		RequestID:     event.RequestID,
		OccurredAt:    occurredAt.Format(time.RFC3339Nano),
		Payload:       json.RawMessage(event.Payload),
	}, nil
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
