# media-worker Implementation Plan

Tai lieu nay chi theo doi rieng `media-worker`. Dung file nay khi can tiep tuc phat trien video processing worker ma khong muon bi lan sang plan tong the cua du an.

Legend:

- `[x]` Done
- `[~]` In progress / partial
- `[ ]` Not started

## Service Purpose

`media-worker` xu ly video bat dong bo sau khi `video-service` xac nhan upload thanh cong.

Service nay phuc vu hai muc tieu:

- Tao product flow that: `uploaded -> processing -> ready/failed`.
- Tao operational evidence cho AIOps/RCA: queue lag, retry, dead-letter, MinIO loi, FFmpeg loi, worker crash, update status fail.

`media-worker` so huu processing jobs, attempts, retry va dead-letter. Service nay khong so huu canonical video metadata va khong doc database cua `video-service`.

## Current Snapshot

As of 2026-07-03:

- `[x]` Da co Go module skeleton.
- `[x]` Da co `cmd/server/main.go` voi `/healthz`, `/readyz`, `/metrics` placeholder.
- `[x]` Da co folder layout theo huong production: `internal/config`, `internal/domain`, `internal/event`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, `tests`.
- `[x]` Da co config loader/validation rieng.
- `[x]` Da co domain model cho processing job/attempt/dead-letter.
- `[x]` Da co PostgreSQL persistence cho job, attempt, inbox va dead-letter.
- `[x]` Da consume `video.uploaded.v1` khi `CONSUMER_ENABLED=true`.
- `[x]` Da co worker loop, retry/backoff va dead-letter.
- `[x]` Da update video status qua `video-service` khi `RUNNER_ENABLED=true`.
- `[x]` Da co MinIO raw object metadata check, raw object download, processed output upload va thumbnail upload.
- `[x]` Da co processing placeholder.
- `[~]` Da co metrics/logs cho consumer, runner, retry/dead-letter; chua co FFmpeg-specific metrics.

## Boundary

`media-worker` owns:

- Processing jobs.
- Processing attempts.
- Dead letters.
- Worker lease/lock state.
- Processing runtime evidence.

`media-worker` may call:

- Redpanda/Kafka to consume `video.uploaded.v1`.
- MinIO/S3-compatible object storage to read raw video and write processed assets/thumbnails.
- `video-service` internal status API to mark `processing`, `ready`, or `failed`.

`media-worker` must not:

- Read or write `video-service` database.
- Own canonical video title/visibility/status outside its own job view.
- Expose public user-facing video APIs.
- Silently mutate product state without status history, logs, metrics, or event evidence.

## Target Runtime Model

Process roles:

- HTTP server exposes `/healthz`, `/readyz`, `/metrics`.
- Kafka consumer receives `video.uploaded.v1`.
- Job runner claims queued/retryable jobs from PostgreSQL.
- Processor executes placeholder processing first, then FFmpeg when ready.
- Status client calls `video-service` internal API with `X-Internal-Token`.

Recommended first deploy shape:

- One binary.
- HTTP server and background worker loops in the same process.
- Config can disable consumer or runner separately for local testing.
- PostgreSQL remains source of truth for job state.

## Event Contracts

Incoming event:

- `[x]` `video.uploaded.v1`

Expected envelope:

- `event_id`
- `event_name`
- `event_version`
- `event_type`
- `aggregate_id`
- `producer`
- `environment`
- `correlation_id`
- `request_id`
- `occurred_at`
- `payload`

Expected payload:

- `video_id`
- `owner_id`
- `raw_object_key`
- `content_type`
- `size_bytes`

Outgoing events:

- `[ ]` `video.processing_started.v1`
- `[ ]` `video.ready.v1`
- `[ ]` `video.processing_failed.v1`

Do not include presigned URLs, internal tokens, MinIO credentials, raw FFmpeg command with secrets, or large stderr blobs in events.

## State Machines

Processing job lifecycle:

```text
queued -> running -> succeeded
   |        |
   |        +-> retrying -> running
   |        |
   |        +-> failed -> dead_letter
   |
   +-> cancelled
```

Attempt lifecycle:

```text
running -> succeeded
running -> failed
```

Video status effect:

```text
video-service: uploaded -> processing -> ready
video-service: uploaded -> processing -> failed
```

Rules:

- A consumed `video.uploaded.v1` should create at most one active processing job for a video.
- Job retry state must be durable in PostgreSQL.
- Dead-letter must preserve enough sanitized context for RCA.
- Worker must be able to resume after crash without losing job intent.
- Updating `video-service` status must use API/event contract, not direct DB write.

## Data Ownership Target

PostgreSQL tables owned by `media-worker`:

- `[x]` `processing_jobs`
- `[x]` `processing_attempts`
- `[x]` `dead_letters`
- `[x]` `inbox_events` or equivalent event idempotency table if Redis is not enough
- `[ ]` `outbox_events` if `media-worker` publishes lifecycle events

Redis planned keys:

- `[ ]` `media:lock:job:{job_id}`
- `[ ]` `media:idempotency:event:{event_id}`
- `[ ]` `media:queue_lag:{queue_name}`

MinIO ownership:

- Reads raw object from bucket/key provided by `video.uploaded.v1`.
- Writes processed object and thumbnail under media-worker-owned output prefixes.
- Stores object keys and processing metadata in its own job/attempt records.
- Sends processed object references to `video-service` only through a controlled contract when that API exists.

## API Surface

Direct worker routes:

- `[x]` `GET /healthz`
- `[x]` `GET /readyz`
- `[x]` `GET /metrics`
- `[ ]` Optional `GET /v1/processing-jobs/{job_id}` for internal/admin debugging.
- `[ ]` Optional `GET /v1/processing-jobs?video_id=&status=&limit=`.

Public clients should not call `media-worker` directly. Admin/ops views should go through `api-gateway` if exposed later.

## Config Target

Required or planned env vars:

- `[x]` `PORT`
- `[x]` `ENVIRONMENT`
- `[x]` `LOG_LEVEL`
- `[x]` `DATABASE_URL`
- `[x]` `KAFKA_BROKERS`
- `[x]` `VIDEO_EVENTS_TOPIC`
- `[x]` `MEDIA_EVENTS_TOPIC`
- `[x]` `CONSUMER_GROUP`
- `[x]` `CONSUMER_ENABLED`
- `[x]` `RUNNER_ENABLED`
- `[x]` `WORKER_ID`
- `[x]` `MAX_ATTEMPTS`
- `[x]` `JOB_LEASE_TTL`
- `[x]` `JOB_POLL_INTERVAL`
- `[x]` `JOB_BATCH_SIZE`
- `[x]` `MINIO_ENDPOINT`
- `[x]` `MINIO_ACCESS_KEY`
- `[x]` `MINIO_SECRET_KEY`
- `[x]` `MINIO_REGION`
- `[x]` `MINIO_USE_SSL`
- `[x]` `RAW_VIDEO_BUCKET`
- `[x]` `PROCESSED_VIDEO_BUCKET`
- `[x]` `THUMBNAIL_BUCKET`
- `[x]` `VIDEO_SERVICE_BASE_URL`
- `[x]` `INTERNAL_API_TOKEN`
- `[x]` `PROCESSING_MODE` (`placeholder` first, `ffmpeg` later)
- `[x]` `FFMPEG_PATH`
- `[x]` `FFPROBE_PATH`
- `[x]` `REQUEST_BODY_LIMIT_BYTES`

Validation rules:

- Non-local environments require PostgreSQL, Kafka, MinIO, `VIDEO_SERVICE_BASE_URL`, and `INTERNAL_API_TOKEN`.
- Consumer cannot start without brokers/topic/group.
- Runner cannot start without database and video-service status client.
- FFmpeg mode cannot start without `FFMPEG_PATH` and `FFPROBE_PATH`.

## Phase 1: Production-Shaped Worker Scaffold

- `[x]` Add `internal/config` with env loading, defaults and validation.
- `[x]` Add `internal/domain` with job/attempt/dead-letter models, states, errors and IDs.
- `[x]` Add `internal/observability` with request/correlation middleware, Prometheus text metrics and readiness state.
- `[x]` Replace placeholder `cmd/server/main.go` with graceful shutdown wiring.
- `[x]` Keep local mode explicit and safe.
- `[x]` Add basic unit tests for config and state transition rules.

Done criteria:

- Service starts locally with no external dependencies only when explicitly in local mode.
- `/healthz`, `/readyz`, `/metrics` are real enough for Kubernetes and AIOps evidence.
- Code layout matches the repository production rules.

## Phase 2: PostgreSQL Job Persistence

- `[x]` Add migration `001_processing_schema.sql`.
- `[x]` Create `processing_jobs`.
- `[x]` Create `processing_attempts`.
- `[x]` Create `dead_letters`.
- `[x]` Add optional `inbox_events` for event idempotency.
- `[x]` Implement repository interface and PostgreSQL store.
- `[x]` Add local in-memory repository only for unit/local tests.
- `[x]` Implement transactional job creation from uploaded event.
- `[x]` Implement claim/lease job query with `locked_by`, `locked_until`, `next_run_at`.
- `[x]` Implement attempt start/success/failure updates.
- `[x]` Add repository tests and skipped-by-default PostgreSQL integration harness.

Done criteria:

- A worker crash does not lose queued/running job evidence.
- Duplicate uploaded events do not create duplicate active jobs.
- Retry/dead-letter decisions are auditable in PostgreSQL.

## Phase 3: Kafka Consumer For `video.uploaded.v1`

- `[x]` Add Kafka consumer using pinned dependency.
- `[x]` Parse and validate event envelope.
- `[x]` Validate payload fields.
- `[x]` Insert inbox/idempotency record by `event_id`.
- `[x]` Create queued processing job with request/correlation evidence.
- `[x]` Commit Kafka offset only after durable job creation.
- `[x]` Add metrics for consumed, duplicate, invalid, failed.
- `[x]` Add tests with fake consumer/event handler.

Done criteria:

- `video.uploaded.v1` creates exactly one queued job.
- Invalid events do not crash the worker loop.
- Queue lag and consume failures are visible to metrics/logs.

## Phase 4: Processing Runner

- `[x]` Add runner loop to claim queued/retrying jobs.
- `[x]` Add `processing` status update call to `video-service` before work starts.
- `[x]` Add processing placeholder mode that simulates success/failure deterministically for tests.
- `[~]` Add MinIO raw object metadata/read path.
- `[x]` Add output object key planning for processed video and thumbnail.
- `[x]` Add status update call to `video-service` for `ready` on success.
- `[x]` Add status update call to `video-service` for `failed` on final failure.
- `[x]` Add attempt records with start/end time, exit code, metrics and sanitized error excerpt.
- `[x]` Add tests for success path and failure path.

Done criteria:

- Uploaded video can move to `processing` and then `ready` or `failed`.
- Worker status updates are auditable through video-service status history and media-worker attempts.
- Placeholder mode allows product flow testing before FFmpeg is ready.

## Phase 5: Retry, Backoff And Dead Letter

- `[x]` Define stable error codes: `RAW_OBJECT_NOT_FOUND`, `MINIO_UNAVAILABLE`, `PROCESS_TIMEOUT`, `FFMPEG_FAILED`, `VIDEO_SERVICE_UNAVAILABLE`, `UNKNOWN_PROCESSING_ERROR`.
- `[x]` Add retry policy by error type.
- `[x]` Add exponential or bounded backoff in `next_run_at`.
- `[x]` Move exhausted jobs to `dead_letter`.
- `[x]` Store sanitized payload/context in `dead_letters`.
- `[x]` Add metrics for retry scheduled, final failure and dead-letter.
- `[x]` Add tests for retryable and non-retryable failures.

Done criteria:

- Transient dependency errors retry predictably.
- Permanent failures stop retrying and become visible RCA evidence.
- Retry storm risk is visible in metrics/logs.

## Phase 6: FFmpeg Processing

- `[x]` Add `internal/processor` abstraction.
- `[x]` Keep placeholder processor for tests/local.
- `[x]` Add FFprobe metadata extraction.
- `[x]` Add FFmpeg transcode path for MVP output.
- `[x]` Add thumbnail generation.
- `[x]` Upload processed output and thumbnail to MinIO.
- `[x]` Capture duration, dimensions, output size and sanitized stderr excerpt.
- `[x]` Enforce command timeout.
- `[x]` Add tests around command construction and failure mapping.

Done criteria:

- Worker can process a small sample video in local/demo.
- FFmpeg failures are mapped to stable error codes.
- Large command output is truncated/redacted before storage/logging.

## Phase 7: Outgoing Events And Contracts

- `[x]` Decide if `media-worker` publishes events directly or only updates `video-service`.
- `[~]` If publishing directly, add outbox table and publisher worker.
- `[~]` Emit or trigger `video.processing_started.v1`.
- `[~]` Emit or trigger `video.ready.v1`.
- `[~]` Emit or trigger `video.processing_failed.v1`.
- `[x]` Document event payloads in contracts docs.
- `[x]` Add tests for event payloads.

Decision:

- For the current product slice, `media-worker` updates processing state through the internal `video-service` status API.
- `video-service` remains the canonical video lifecycle owner and event producer for downstream services.
- `media-worker` keeps versioned lifecycle contract builders for `video.processing_started.v1`, `video.ready.v1`, and `video.processing_failed.v1` so a direct publisher/outbox can be added later without inventing payloads under pressure.
- A direct `media-worker` outbox table and Kafka publisher are deferred until there is a clear downstream need that cannot be handled by `video-service` lifecycle events.

Done criteria:

- Downstream services can react to processing lifecycle without database coupling.
- Event payloads have stable versions and evidence fields.

## Phase 8: Observability And Incident Evidence

- `[ ]` Add metrics for jobs by status.
- `[ ]` Add metrics for attempts by outcome/error code.
- `[ ]` Add metrics for retry/dead-letter counts.
- `[ ]` Add metrics for Kafka consume lag or observed event age.
- `[ ]` Add metrics for MinIO read/write latency and errors.
- `[ ]` Add metrics for video-service status update latency/errors.
- `[ ]` Add structured logs with `service`, `environment`, `worker_id`, `job_id`, `attempt_id`, `video_id`, `request_id`, `correlation_id`, `error_code`.
- `[ ]` Add optional OpenTelemetry trace propagation.

Done criteria:

- AIOps can diagnose queue lag, MinIO outage, FFmpeg failure, video-service outage, retry storm and dead-letter spikes.

## Phase 9: Deployment Readiness

- `[x]` Dockerfile exists.
- `[ ]` Add service env documentation for DB, Kafka, MinIO, video-service API and mode.
- `[ ]` Add local compose dependencies when needed.
- `[ ]` Add Kubernetes/GitOps manifests in companion repo when ready.
- `[ ]` Add resource requests/limits suitable for CPU-heavy work.
- `[ ]` Add liveness/readiness probes.
- `[ ]` Add secret/config references without hard-coded credentials.
- `[ ]` Add smoke test for upload event to processing job flow.

Done criteria:

- Service can run in local compose and Kubernetes with the same config model.
- Missing required production dependency fails fast.
- Worker failures generate useful evidence instead of silent data loss.

## Immediate Next Task

Next best engineering task:

1. Add richer Phase 8 observability: queue lag, job status gauges, MinIO latency/error counters and video-service status update latency.
2. Add local compose or CI wiring for media-worker PostgreSQL integration tests.
3. Add a small local sample-video smoke test for `PROCESSING_MODE=ffmpeg`.
4. Add Kubernetes/GitOps manifests after the local worker flow is stable.

Reason:

- `video-service` already writes and publishes `video.uploaded.v1`.
- The worker can now turn raw uploaded objects into processed outputs in FFmpeg mode.
- The next gap is making this flow observable and deployable enough for incident/RCA evidence.
- Consumer, placeholder runner and retry/dead-letter behavior now exist; the next gap is replacing placeholder work with real FFmpeg and output object writes.

## Update Rule

When working on `media-worker`:

- Read `AGENTS.md`, `PROJECT_CONTEXT.md`, `PROJECT_PROGRESS.md`, this file and `services/media-worker/README.md`.
- Update this checklist when a meaningful item changes.
- Update `PROJECT_PROGRESS.md` after substantial implementation work.
- Keep `docs/development/implementation-plan.md` as the high-level roadmap and this file as the detailed `media-worker` roadmap.
