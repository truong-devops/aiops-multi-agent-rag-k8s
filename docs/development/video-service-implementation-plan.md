# video-service Implementation Plan

Tai lieu nay chi theo doi rieng `video-service`. Dung file nay khi can tiep tuc phat trien upload/video lifecycle ma khong muon bi lan sang plan tong the cua du an.

Legend:

- `[x]` Done
- `[~]` In progress / partial
- `[ ]` Not started

## Service Purpose

`video-service` quan ly vong doi video o phan product platform:

- Tao upload request cho creator.
- Luu metadata video va raw object key.
- Xac nhan upload da hoan tat.
- Quan ly video status theo state machine.
- Ghi status history va event outbox de `media-worker` co the xu ly tiep.
- Tao log/metrics/traces du tot de AIOps co bang chung khi phan tich su co.

Service nay khong xu ly FFmpeg, khong quan ly retry worker, khong ghi feed/social data va khong doc database cua service khac.

## Current Snapshot

As of 2026-06-30:

- `[x]` Da co Go service layout theo huong production: `cmd/server`, `internal/config`, `internal/domain`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, `tests`.
- `[x]` Da co domain model cho video, upload request va status history.
- `[x]` Da co state machine cho video status.
- `[x]` Da co in-memory repository de chay local/test.
- `[x]` Da co HTTP API slice dau tien cho upload request, confirm uploaded, get/list video va update status.
- `[x]` Da co `/healthz`, `/readyz`, `/metrics`, request ID va correlation ID middleware.
- `[x]` Da co unit/handler tests cho flow hien tai.
- `[x]` Da co PostgreSQL migration cho `videos`, `upload_requests`, `video_assets`, `video_status_history`, `outbox_events`.
- `[x]` Da co PostgreSQL repository cho video, upload request va status history.
- `[x]` Da co `DATABASE_URL` config va startup wiring chon PostgreSQL khi co DSN.
- `[ ]` Chua co MinIO presigned upload URL that.
- `[x]` Da co outbox event write cho `video.uploaded.v1` khi confirm upload.
- `[ ]` Chua co Redpanda/Kafka publisher.
- `[~]` Da co PostgreSQL integration test harness, chua wire vao CI/local compose.

## Implemented API Surface

Direct service routes:

- `[x]` `POST /v1/videos/upload-requests`
- `[x]` `POST /v1/videos/{video_id}/uploaded`
- `[x]` `GET /v1/videos/{video_id}`
- `[x]` `GET /v1/videos?owner_id=&status=&limit=`
- `[x]` `PATCH /v1/videos/{video_id}/status`
- `[x]` `GET /healthz`
- `[x]` `GET /readyz`
- `[x]` `GET /metrics`

Public clients should reach these through `api-gateway` under `/api/v1/*`.

## State Machines

Video lifecycle:

```text
draft -> uploaded -> processing -> ready
   |         |            |          |
   v         v            v          v
deleted   failed       failed     deleted
             |
             v
          processing
```

Rules:

- `draft` is created when upload request is created.
- `uploaded` means upload metadata is confirmed and `video.uploaded.v1` should be produced.
- `processing`, `ready`, and `failed` should normally be driven by `media-worker` through controlled API/event paths.
- `deleted` is terminal for normal product flows.

Upload request lifecycle:

```text
created -> uploaded
   |
   +--> expired
   |
   +--> cancelled
```

Rules:

- Upload request is short-lived and must have `expires_at`.
- Presigned URL must not be stored in durable tables or logs.
- Confirm upload must be idempotency-aware before the service is considered production-ready.

## Data Ownership Target

PostgreSQL tables owned by `video-service`:

- `[x]` `videos`
- `[x]` `upload_requests`
- `[x]` `video_assets`
- `[x]` `video_status_history`
- `[x]` `outbox_events`

Redis planned keys:

- `[ ]` `video:upload_intent:{upload_request_id}`
- `[ ]` `video:object_meta:{video_id}`
- `[ ]` `video:idempotency:{request_id}`

MinIO ownership:

- `video-service` decides raw bucket and raw object key.
- `media-worker` creates processed media and thumbnails later.
- Database stores object keys, not binary data and not presigned URLs.

## Event Contracts

Main outgoing event:

- `[x]` `video.uploaded.v1` is written to `outbox_events` as `pending`.

Payload must include:

- `video_id`
- `owner_id`
- `raw_object_key`
- `content_type`
- `size_bytes`

Envelope must include:

- `event_id`
- `event_name`
- `event_version`
- `occurred_at`
- `producer`
- `environment`
- `correlation_id`
- `request_id`

Do not include presigned URLs, tokens, credentials, or internal secrets in events.

## Phase 1: Harden Current In-Memory Slice

- `[x]` Keep business logic out of `cmd/server`.
- `[x]` Keep HTTP handlers thin and push workflow logic into service layer.
- `[x]` Validate title, content type, visibility and size.
- `[x]` Enforce video state transition rules.
- `[x]` Add request/correlation ID to responses and entity state.
- `[x]` Add basic metrics.
- `[x]` Add tests for upload request and state transition flow.
- `[ ]` Align upload confirmation route with final REST API naming if needed: current code uses `/uploaded`, docs also mention `/upload-completions`.
- `[ ]` Add authorization checks for owner-only video mutation when gateway user context is finalized.

Done criteria:

- Service can run locally without dependencies.
- Tests cover the current API behavior.
- In-memory mode is clearly local/test only.

## Phase 2: PostgreSQL Persistence

- `[x]` Add `migrations/001_video_schema.sql`.
- `[x]` Add `DATABASE_URL` config and validation.
- `[x]` Add PostgreSQL connection pool with timeouts.
- `[x]` Implement `repository.PostgresStore`.
- `[x]` Implement transactional `CreateVideoWithUploadRequest`.
- `[x]` Implement transactional upload confirmation.
- `[x]` Implement transactional status update with status history.
- `[x]` Add DB indexes from `docs/architecture/database-design.md`.
- `[x]` Keep in-memory repository only for local/test mode.
- `[~]` Add integration tests for repository behavior.

Done criteria:

- Video metadata, upload requests and status history survive process restart.
- `/readyz` fails when required DB dependency is unavailable outside local-only mode.
- Invalid state transitions remain rejected at service layer.

## Phase 3: MinIO Upload Integration

- `[ ]` Add MinIO/S3-compatible client configuration.
- `[ ]` Add required config: endpoint, access key, secret key, raw bucket, URL expiry.
- `[ ]` Generate presigned PUT URL for upload request.
- `[ ]` Ensure bucket/object key are stored, presigned URL is not persisted.
- `[ ]` Validate content type and expected size before creating upload intent.
- `[ ]` Optionally verify object metadata when confirming upload.
- `[ ]` Add failure mapping for MinIO unavailable, access denied and bucket missing.

Done criteria:

- Creator receives a real upload URL.
- Raw object can be uploaded to MinIO.
- Operational failures produce stable error codes and useful logs.

## Phase 4: Outbox And Event Publishing

- `[x]` Write `video.uploaded.v1` into `outbox_events` in the same DB transaction as upload confirmation.
- `[x]` Add outbox event domain model.
- `[x]` Add event payload builder.
- `[ ]` Add publisher worker for Redpanda/Kafka-compatible broker.
- `[ ]` Mark outbox event `published` only after broker ack.
- `[ ]` Add retry/backoff and `failed` status for publish failures.
- `[ ]` Add metrics for pending, published and failed outbox events.
- `[~]` Add tests proving upload confirmation and outbox write are atomic.

Done criteria:

- `media-worker` can consume `video.uploaded.v1`.
- Publish failure does not lose event intent.
- AIOps can see event backlog/publish failures as incident evidence.

## Phase 5: media-worker Integration

- `[ ]` Define exact contract for `media-worker` to mark processing started, ready or failed.
- `[ ]` Decide controlled update path: service API command, event command, or both with clear ownership.
- `[ ]` Protect internal status update endpoint from public clients.
- `[ ]` Add status reason and stable error code handling.
- `[ ]` Add tests for worker-driven status transitions.

Done criteria:

- Uploaded video creates a processing path.
- Worker updates are auditable through status history, logs and request/correlation IDs.
- Failed processing leaves enough evidence for RCA.

## Phase 6: Observability And Incident Evidence

- `[x]` Add request counter and duration metrics.
- `[x]` Preserve request ID and correlation ID.
- `[ ]` Add structured logs with `service`, `environment`, `request_id`, `correlation_id`, `video_id`, `upload_request_id`.
- `[ ]` Add metrics for upload request created/uploaded/expired.
- `[ ]` Add metrics for video status transitions.
- `[ ]` Add metrics for DB latency/errors.
- `[ ]` Add metrics for MinIO presign/metadata failures.
- `[ ]` Add metrics for outbox pending/published/failed.
- `[ ]` Add optional OpenTelemetry trace propagation.

Done criteria:

- AIOps can diagnose DB outage, MinIO outage, event publish failure and invalid state transition spikes from logs/metrics.

## Phase 7: Deployment Readiness

- `[x]` Dockerfile exists.
- `[ ]` Add service env documentation for DB, MinIO, broker and mode.
- `[ ]` Add Kubernetes/GitOps manifests in companion repo when ready.
- `[ ]` Add resource requests/limits.
- `[ ]` Add liveness/readiness probes.
- `[ ]` Add secret/config references without hard-coded credentials.
- `[ ]` Add smoke test script for upload request flow.

Done criteria:

- Service can run in local compose and Kubernetes with the same config model.
- Missing required production dependency fails fast.
- No secrets or presigned URLs appear in logs.

## Immediate Next Task

Next best engineering task:

1. Wire PostgreSQL integration tests into local compose or CI.
2. Add an outbox publisher worker for Redpanda/Kafka.
3. Add MinIO presigned upload URL generation.
4. Add idempotency handling for upload request creation.

Reason:

- PostgreSQL persistence and outbox write now have a DB test harness, but it still needs a repeatable local/CI database workflow.
- The publisher is the next step that turns pending outbox rows into consumable broker events for `media-worker`.
- MinIO presigned upload should follow once durable metadata and event intent are stable.

## Update Rule

When working on `video-service`:

- Read `AGENTS.md`, `PROJECT_CONTEXT.md`, `PROJECT_PROGRESS.md`, this file and `services/video-service/README.md`.
- Update this checklist when a meaningful item changes.
- Update `PROJECT_PROGRESS.md` after substantial implementation work.
- Keep `docs/development/implementation-plan.md` as the high-level roadmap and this file as the detailed `video-service` roadmap.
