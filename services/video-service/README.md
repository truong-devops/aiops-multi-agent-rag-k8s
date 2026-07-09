# video-service

Service quản lý video metadata, upload flow và event phát sang media worker.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Tạo upload request.
- Lưu video metadata.
- Sinh raw object key và presigned upload URL cho upload flow.
- Confirm upload hoàn tất và chuyển video sang trạng thái `uploaded`.
- Cập nhật trạng thái video theo state machine.
- Ghi outbox event và publish sang Redpanda/Kafka khi bật worker.
- Expose health, readiness và metrics.

## API

Direct service paths:

- `POST /v1/videos/upload-requests`
- `POST /v1/videos/{video_id}/uploaded`
- `GET /v1/videos/{video_id}`
- `GET /v1/videos?owner_id=&status=&limit=`
- `PATCH /v1/videos/{video_id}/status`
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Public paths should be reached through `api-gateway` as `/api/v1/...`.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP server port. |
| `ENVIRONMENT` | `local` | Runtime environment label. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `DATABASE_URL` | empty | PostgreSQL DSN. Required outside local/dev/test environments. |
| `INTERNAL_API_TOKEN` | empty | Shared internal token for worker/internal status updates. Required outside local/dev/test environments. |
| `RAW_VIDEO_BUCKET` | `raw-videos` | Bucket name used for raw upload object keys. |
| `UPLOAD_URL_BASE` | empty | Optional local/dev upload base URL fallback when MinIO presigner is not configured. |
| `UPLOAD_REQUEST_TTL` | `30m` | Upload request expiry duration. |
| `PRESIGNED_UPLOAD_TTL` | `15m` | Presigned PUT URL expiry duration. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | Max request body size. |
| `MINIO_ENDPOINT` | empty | S3-compatible endpoint, for example `minio:9000`. When set, the service returns real SigV4 presigned PUT URLs. |
| `MINIO_ACCESS_KEY` | empty | MinIO/S3 access key. Required when `MINIO_ENDPOINT` is set. |
| `MINIO_SECRET_KEY` | empty | MinIO/S3 secret key. Required when `MINIO_ENDPOINT` is set. |
| `MINIO_REGION` | `us-east-1` | S3 signing region. |
| `MINIO_USE_SSL` | `false` | Use HTTPS for presigned URLs. |
| `VERIFY_UPLOAD_OBJECT` | `false` | When true, confirm upload verifies raw object metadata through S3-compatible HEAD before marking video uploaded. Requires `MINIO_ENDPOINT`. |
| `KAFKA_BROKERS` | empty | Comma-separated Redpanda/Kafka broker list. Required when outbox publisher is enabled. |
| `VIDEO_EVENTS_TOPIC` | `video.events` | Kafka topic for video lifecycle events. |
| `OUTBOX_PUBLISHER_ENABLED` | `false` | Enables the outbox publisher worker. |
| `OUTBOX_POLL_INTERVAL` | `5s` | Outbox polling interval. |
| `OUTBOX_BATCH_SIZE` | `25` | Max outbox events processed per poll. |

## Current Implementation

The service can run with either PostgreSQL or an in-memory repository.

- When `DATABASE_URL` is set, the service uses PostgreSQL for videos, upload requests and status history.
- When `DATABASE_URL` is empty in local/dev/test environments, the service falls back to an in-memory store for local development.
- Outside local/dev/test environments, `DATABASE_URL` is required and startup fails fast if it is missing.

Implemented integration foundation:

- Upload request creation supports `Idempotency-Key` per owner so retried client requests reuse the same upload intent.
- When MinIO config is present, upload request creation returns a real S3-compatible SigV4 presigned PUT URL.
- When `VERIFY_UPLOAD_OBJECT=true`, confirm upload checks the raw object exists and verifies size/content type metadata before marking the video uploaded.
- Confirm upload writes `video.uploaded.v1` into `outbox_events` in the same repository operation as upload/video status updates.
- Public videos that transition to `ready` write `video.ready.v1` into `outbox_events` in the same repository operation as the status update, so `feed-social-service` can ingest the ready video from Kafka/Redpanda.
- When `OUTBOX_PUBLISHER_ENABLED=true`, the outbox worker publishes envelopes to Redpanda/Kafka and marks events `published` only after broker ack.
- Owner/admin/internal authorization is enforced for read, confirm and status update paths. Worker-driven status updates should use `X-Internal-Token`.
- `/metrics` includes HTTP, upload, presign, object verification, status transition, outbox publish and DB operation counters.

Production integration work still needs:

- Richer retry backoff controls for outbox failures.
- Redis cache for short-lived upload intent/object metadata if the flow needs it later.

## State Machine

Video states:

```text
draft -> uploaded -> processing -> ready
                         └-------> failed
```

The service rejects invalid state transitions.

Upload request states:

```text
created -> uploaded
   └----> expired
   └----> cancelled
```

## Event Plan

The service records `video.uploaded.v1` as a pending outbox event after upload metadata is committed. It also records `video.ready.v1` when a public video reaches `ready`. The event contract is defined in `packages/contracts/event-contracts.md`.

The current implementation records request ID, correlation ID, producer, environment and payload fields so the outbox publisher emits traceable event envelopes.

## Trách Nhiệm Chưa Làm

- Add Kubernetes/GitOps manifests for DB, MinIO, Redpanda and service secrets.
- Add smoke test script for the end-to-end upload request flow.

## Tests

```bash
go test ./...
```

PostgreSQL repository integration tests are skipped by default. To run them, provide a disposable database URL:

```bash
VIDEO_SERVICE_TEST_DATABASE_URL='postgres://video:video@localhost:5432/video_db?sslmode=disable' go test ./internal/repository
```

Or use the local compose test database:

```bash
make test-video-integration
```

## Dependencies Dự Kiến

- PostgreSQL for video metadata, upload requests, assets and outbox events.
- Redis for short-lived upload intent cache and idempotency keys.
- MinIO.
- Redpanda/Kafka.

## Incident Có Thể Sinh

- MinIO AccessDenied.
- Publish event fail.
- DB connection pool cạn.
- Deploy thiếu env/secret.
