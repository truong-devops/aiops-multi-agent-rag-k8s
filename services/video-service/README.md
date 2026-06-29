# video-service

Service quản lý video metadata, upload flow và event phát sang media worker.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Tạo upload request.
- Lưu video metadata.
- Sinh raw object key cho upload flow.
- Confirm upload hoàn tất và chuyển video sang trạng thái `uploaded`.
- Cập nhật trạng thái video theo state machine.
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
| `RAW_VIDEO_BUCKET` | `raw-videos` | Bucket name used for raw upload object keys. |
| `UPLOAD_URL_BASE` | empty | Optional local/dev upload base URL. This is not a real presigned URL implementation. |
| `UPLOAD_REQUEST_TTL` | `30m` | Upload request expiry duration. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | Max request body size. |

## Current Implementation

The service can run with either PostgreSQL or an in-memory repository.

- When `DATABASE_URL` is set, the service uses PostgreSQL for videos, upload requests and status history.
- When `DATABASE_URL` is empty in local/dev/test environments, the service falls back to an in-memory store for local development.
- Outside local/dev/test environments, `DATABASE_URL` is required and startup fails fast if it is missing.

Production integration work still needs:

- MinIO presigned upload URL generation.
- Redpanda/Kafka event publishing for `video.uploaded`.
- Outbox writes/publisher for `video.uploaded.v1`.
- Redis idempotency cache for upload request creation.

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

The service should publish `video.uploaded.v1` after upload metadata is committed. The event contract is defined in `packages/contracts/event-contracts.md`.

This first implementation records request and correlation IDs on video/upload state so the future outbox publisher has the right evidence fields.

## Trách Nhiệm Chưa Làm

- Tích hợp MinIO/presigned URL thật.
- Publish event `video.uploaded`.
- Outbox write and publisher worker.
- Database-backed integration tests.

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
