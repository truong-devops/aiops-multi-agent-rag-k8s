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
| `PROCESSED_VIDEO_BUCKET` | `processed-videos` | Processed MP4 output bucket. |
| `THUMBNAIL_BUCKET` | `thumbnails` | Thumbnail output bucket. |
| `VIDEO_SERVICE_BASE_URL` | empty | Internal video-service base URL. Required outside local/dev/test and when `RUNNER_ENABLED=true`. |
| `INTERNAL_API_TOKEN` | empty | Token sent to video-service internal status API. Required outside local/dev/test and when `RUNNER_ENABLED=true`. |
| `PROCESSING_MODE` | `placeholder` | `placeholder` for local flow checks or `ffmpeg` for real processing. |
| `FFMPEG_PATH` | `ffmpeg` | FFmpeg binary path for FFmpeg mode. |
| `FFPROBE_PATH` | `ffprobe` | FFprobe binary path for FFmpeg mode. |
| `PROCESSING_TIMEOUT` | `30m` | Per-attempt FFmpeg/FFprobe processing timeout. |
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
- FFmpeg processor mode that downloads the raw object from MinIO/S3, extracts metadata with FFprobe, transcodes an MP4 output, generates a JPEG thumbnail, uploads both outputs, and records processing metrics.
- HTTP client for video-service internal status updates through `X-Internal-Token`.
- Retry/backoff policy and stable processing error codes.
- Operational metrics for job status, runnable queue age/depth, attempt outcome/error code, database operations, MinIO operations, video-service status updates and observed `video.uploaded.v1` event age.
- Structured logs that include service, environment, worker, job, attempt, video, request, correlation and error-code context on worker paths.
- Lifecycle event contract builders for `video.processing_started.v1`, `video.ready.v1`, and `video.processing_failed.v1`; direct worker publishing is deferred while `video-service` remains the canonical lifecycle event producer. On successful processing, the worker sends processed asset metadata to `video-service`, which publishes the canonical `video.ready.v1` event.
- Unit tests and skipped-by-default PostgreSQL integration harness.
- FFmpeg smoke test behind the `smoke` build tag.

Still pending:

- Direct media-worker lifecycle outbox/publisher only if downstream requirements need worker-owned lifecycle events beyond the canonical `video-service` lifecycle stream.
- Kubernetes/GitOps manifests and resource sizing for CPU-heavy FFmpeg work.
- Full compose smoke test from upload event to processing status update.
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

Run the FFmpeg processor smoke test with a generated sample video:

```bash
go test -tags smoke ./internal/processor -run TestFFmpegProcessorSmoke -count=1
```

PostgreSQL repository integration tests are skipped by default. To run them, provide a disposable database URL:

```bash
MEDIA_WORKER_TEST_DATABASE_URL='postgres://media:media@localhost:5432/media_db?sslmode=disable' go test ./internal/repository
```

From the repository root, the same checks are available as:

```bash
make test-media
make test-media-integration
make smoke-media-ffmpeg
```
