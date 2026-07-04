package client

import (
	"context"
	"time"
)

type MetricsRecorder interface {
	RecordDependencyOperation(dependency string, operation string, outcome string, duration time.Duration)
}

type InstrumentedVideoStatusClient struct {
	next    VideoStatusClient
	metrics MetricsRecorder
	now     func() time.Time
}

func NewInstrumentedVideoStatusClient(next VideoStatusClient, metrics MetricsRecorder) VideoStatusClient {
	return InstrumentedVideoStatusClient{next: next, metrics: metrics, now: time.Now}
}

func (c InstrumentedVideoStatusClient) UpdateStatus(ctx context.Context, input UpdateVideoStatusInput) error {
	startedAt := c.now()
	err := c.next.UpdateStatus(ctx, input)
	if c.metrics != nil {
		outcome := input.Status
		if err != nil {
			outcome = "error"
		}
		c.metrics.RecordDependencyOperation("video-service", "update_status", outcome, c.now().Sub(startedAt))
	}
	return err
}
