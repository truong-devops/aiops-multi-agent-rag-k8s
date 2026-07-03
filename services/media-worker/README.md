# media-worker

Worker xử lý video bất đồng bộ bằng FFmpeg.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Consume event `video.uploaded`.
- Tải file từ MinIO.
- Chạy FFmpeg.
- Tạo thumbnail.
- Cập nhật trạng thái video qua `video-service`.
- Lưu processing job, attempt, retry và dead-letter evidence.
- Retry và dead-letter queue.
- Expose health, readiness và metrics.

## API

Direct worker paths:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Public clients should not call `media-worker` directly.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP server port. |
| `ENVIRONMENT` | `local` | Runtime environment label. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `DATABASE_URL` | empty | PostgreSQL DSN. Required outside local/dev/test and when `RUNNER_ENABLED=true`. |
| `KAFKA_BROKERS` | empty | Comma-separated Redpanda/Kafka broker list. Required outside local/dev/test and when `CONSUMER_ENABLED=true`. |
| `VIDEO_EVENTS_TOPIC` | `video.events` | Topic containing `video.uploaded.v1`. |
| `MEDIA_EVENTS_TOPIC` | `media.events` | Future media lifecycle event topic. |
| `CONSUMER_GROUP` | `media-worker` | Kafka consumer group. |
| `CONSUMER_ENABLED` | `false` | Enables the `video.uploaded.v1` consumer. |
| `RUNNER_ENABLED` | `false` | Enables the job runner loop. |
| `WORKER_ID` | hostname | Worker identity used for job leases and attempt records. |
| `MAX_ATTEMPTS` | `3` | Max processing attempts before final failure/dead-letter. |
| `JOB_LEASE_TTL` | `2m` | Job claim lease duration. |
| `JOB_POLL_INTERVAL` | `5s` | Future runner poll interval. |
| `JOB_BATCH_SIZE` | `10` | Max jobs claimed per poll. |
| `MINIO_ENDPOINT` | empty | S3-compatible endpoint. Required outside local/dev/test. |
| `MINIO_ACCESS_KEY` | empty | MinIO/S3 access key. Required outside local/dev/test. |
| `MINIO_SECRET_KEY` | empty | MinIO/S3 secret key. Required outside local/dev/test. |
| `MINIO_REGION` | `us-east-1` | S3 signing region. |
| `MINIO_USE_SSL` | `false` | Use HTTPS for object storage calls. |
| `RAW_VIDEO_BUCKET` | `raw-videos` | Raw input bucket. |
| `PROCESSED_VIDEO_BUCKET` | `processed-videos` | Future processed output bucket. |
| `THUMBNAIL_BUCKET` | `thumbnails` | Future thumbnail bucket. |
| `VIDEO_SERVICE_BASE_URL` | empty | Internal video-service base URL. Required outside local/dev/test and when `RUNNER_ENABLED=true`. |
| `INTERNAL_API_TOKEN` | empty | Token sent to video-service internal status API. Required outside local/dev/test and when `RUNNER_ENABLED=true`. |
| `PROCESSING_MODE` | `placeholder` | `placeholder` first, `ffmpeg` later. |
| `FFMPEG_PATH` | `ffmpeg` | FFmpeg binary path for future FFmpeg mode. |
| `FFPROBE_PATH` | `ffprobe` | FFprobe binary path for future FFmpeg mode. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | Max HTTP request body size. |

## Current Implementation

Implemented now:

- Production-shaped config loader and startup validation.
- Graceful HTTP server with request/correlation ID middleware.
- `/healthz`, `/readyz`, and Prometheus text `/metrics`.
- Domain models and state rules for processing jobs, attempts, inbox events, and dead letters.
- PostgreSQL migration `001_processing_schema.sql`.
- PostgreSQL store and local in-memory store for:
  - idempotent job creation from uploaded events,
  - job list/find,
  - runnable job claim/lease,
  - attempt start/success/failure,
  - dead-letter persistence.
- Kafka consumer for `video.uploaded.v1` with envelope parsing, validation, durable job creation and commit-after-persist behavior.
- Placeholder runner that claims jobs, verifies the raw object through S3-compatible HEAD, updates `video-service` status to `processing`, then marks the job `succeeded`, `retrying`, or `dead_letter`.
- HTTP client for video-service internal status updates through `X-Internal-Token`.
- Retry/backoff policy and stable processing error codes.
- Unit tests and skipped-by-default PostgreSQL integration harness.

Still pending:

- Real FFmpeg/FFprobe processing.
- Processed video and thumbnail upload to MinIO.
- Outgoing media lifecycle events.
- Redis locks/idempotency cache, if needed after PostgreSQL behavior is stable.

## Dependencies Dự Kiến

- Redpanda/Kafka.
- MinIO.
- PostgreSQL for processing jobs, attempts and dead letters.
- Redis for distributed job locks and idempotency keys.
- FFmpeg.

## Incident Có Thể Sinh

- OOMKilled khi xử lý video lớn.
- Queue lag tăng.
- FFmpeg lỗi.
- Retry storm.
- MinIO latency cao.

## Tests

```bash
go test ./...
```

PostgreSQL repository integration tests are skipped by default. To run them, provide a disposable database URL:

```bash
MEDIA_WORKER_TEST_DATABASE_URL='postgres://media:media@localhost:5432/media_db?sslmode=disable' go test ./internal/repository
```
