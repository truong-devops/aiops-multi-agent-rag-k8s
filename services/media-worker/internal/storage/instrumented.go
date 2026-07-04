package storage

import (
	"context"
	"io"
	"time"
)

type MetricsRecorder interface {
	RecordDependencyOperation(dependency string, operation string, outcome string, duration time.Duration)
}

type InstrumentedObjectStore struct {
	next    ObjectStore
	metrics MetricsRecorder
	now     func() time.Time
}

func NewInstrumentedObjectStore(next ObjectStore, metrics MetricsRecorder) ObjectStore {
	return InstrumentedObjectStore{next: next, metrics: metrics, now: time.Now}
}

func (s InstrumentedObjectStore) VerifyObject(ctx context.Context, input VerifyObjectInput) (ObjectMetadata, error) {
	startedAt := s.now()
	out, err := s.next.VerifyObject(ctx, input)
	s.record("verify_object", err, startedAt)
	return out, err
}

func (s InstrumentedObjectStore) DownloadObject(ctx context.Context, input ObjectRef) (io.ReadCloser, error) {
	startedAt := s.now()
	out, err := s.next.DownloadObject(ctx, input)
	s.record("download_object", err, startedAt)
	return out, err
}

func (s InstrumentedObjectStore) UploadObject(ctx context.Context, input UploadObjectInput) (ObjectMetadata, error) {
	startedAt := s.now()
	out, err := s.next.UploadObject(ctx, input)
	s.record("upload_object", err, startedAt)
	return out, err
}

func (s InstrumentedObjectStore) record(operation string, err error, startedAt time.Time) {
	if s.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	s.metrics.RecordDependencyOperation("minio", operation, outcome, s.now().Sub(startedAt))
}
