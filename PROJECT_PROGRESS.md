# Project Progress

This file is a living handoff log for future coding sessions and other AI agents.

Use it to understand what has already been decided or done before starting new work. It is not a replacement for git history, tests, or source inspection. Always verify the current code and `git status` before making changes.

## How To Use This File

Before coding:

1. Read `AGENTS.md`.
2. Read `PROJECT_CONTEXT.md`.
3. Read this file.
4. Read the relevant architecture, API, service, and development docs for the task.
5. Inspect the real code and `git status`.

After substantial work:

- Add a dated entry under `Work Log`.
- Update `Current State` if implementation status changed.
- Update `Next Useful Steps` if priorities changed.
- Record new architecture decisions under `Decisions Made`.
- Record unresolved risks under `Open Questions / Risks`.

Keep entries concise. This file should help the next session continue, not become a full changelog.

## Current State

As of 2026-07-03:

- The project direction is set: a Kubernetes-based microservices video/livestream platform used as a realistic testbed for AIOps RCA with Multi-Agent RAG, DevSecOps evidence, and GitOps-safe remediation.
- The thesis framing is centered on Multi-Agent RAG for incident investigation and root cause analysis, not on building a commercial video product for its own sake.
- Source code and product docs live in this repository.
- Kubernetes desired state is intended to live in the companion GitOps repository at `../aiops-gitops-manifests`.
- Product Go services have a consistent scaffold: `cmd/server`, `internal/config`, `internal/domain`, `internal/event`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, and `tests`.
- `identity-service` and `api-gateway` are more implemented than the other product services.
- `api-gateway` now has route proxying, request/correlation IDs, CORS, body limits, upstream timeout, JWT verification through identity JWKS, trusted user-context forwarding, internal header stripping, JSON gateway/auth errors, readiness checks, and basic Prometheus text metrics.
- `video-service` now has a production-shaped implementation for upload requests, video metadata, upload confirmation with optional MinIO/S3 object metadata verification, video status transitions, request/correlation IDs, readiness, metrics, tests, PostgreSQL persistence with local in-memory fallback, local/CI DB integration workflow, idempotent upload intent creation, MinIO/S3 presigned upload URLs, owner/internal authorization, pending outbox writes for `video.uploaded.v1`, and a Redpanda/Kafka outbox publisher worker.
- `media-worker` now has a production-shaped scaffold, config validation, health/readiness/metrics, domain models for processing jobs/attempts/dead letters, PostgreSQL schema and repository, local in-memory test store, Kafka consumer for `video.uploaded.v1`, placeholder processing runner, FFmpeg/FFprobe processing mode, MinIO raw download/output upload, thumbnail generation, video-service internal status update client, retry/backoff, dead-letter behavior, and lifecycle event contract builders.
- Several product services beyond `identity-service`, `api-gateway`, `video-service`, and the first `media-worker` foundation are still mostly skeletons with health, readiness, and metrics placeholders.
- `aiops-service` has a Python package layout for future collectors, agents, RAG, scoring, redaction, schemas, and API work.

## Decisions Made

- Code should be production-shaped by default, even during thesis development.
- The app is a realistic operational testbed, not an infinite product surface.
- Product feature work must follow `docs/development/product-code-rules.md`.
- Service boundaries from `docs/architecture/service-boundaries.md` are authoritative.
- Data ownership from `docs/architecture/data-ownership.md` is authoritative.
- Database strategy from `docs/architecture/database-design.md` is authoritative.
- Public client traffic should go through `api-gateway` under `/api/v1/*`.
- Services must not read each other's databases directly.
- Redis must not be used as a durable source of truth.
- Remediation should be advisory or GitOps-proposal-based by default, not direct production mutation.
- New product code should preserve clear layers: handler, service, repository, domain, event, observability.
- New AIOps code should preserve clear layers: API, core config, collectors, agents, RAG, redaction, scoring, schemas.
- `video-service` remains the canonical video lifecycle event producer for now; `media-worker` updates status through the internal API and keeps lifecycle event contracts ready for a future direct outbox only if needed.

## Work Log

### 2026-07-03

- Implemented `media-worker` Phase 6 and Phase 7 contract work from `docs/development/media-worker-implementation-plan.md`.
- Added FFmpeg/FFprobe processing mode with raw object download, MP4 transcode, thumbnail generation, processed/thumbnail object uploads, command timeout, sanitized stderr failure mapping, and tests using fake command/object-storage adapters.
- Added S3-compatible object download/upload support in the media-worker object store.
- Added lifecycle event contract builders and tests for `video.processing_started.v1`, `video.ready.v1`, and `video.processing_failed.v1`; direct worker publishing remains deferred while `video-service` owns canonical lifecycle events.
- Added `docs/api/event-contracts.md` and updated media-worker/implementation docs.
- Notes for next session: run/keep the media-worker tests green, then add Phase 8 observability metrics and a local sample-video smoke test for `PROCESSING_MODE=ffmpeg`.

- Implemented `media-worker` Phase 3, Phase 4 and Phase 5 from `docs/development/media-worker-implementation-plan.md`.
- Added `github.com/segmentio/kafka-go v0.4.51`, a `video.uploaded.v1` envelope parser, and a Kafka consumer worker that commits offsets only after durable job registration.
- Added a video-service internal status client for `processing`, `ready`, and `failed` updates through `X-Internal-Token`.
- Added a placeholder processor, S3-compatible raw object metadata verification, runner loop, retry/backoff decisions, dead-letter payload creation, and tests for success, retry and dead-letter paths.
- Wired consumer and runner startup through `CONSUMER_ENABLED` and `RUNNER_ENABLED`.
- Updated `services/media-worker/README.md`, `docs/development/media-worker-implementation-plan.md`, `docs/development/implementation-plan.md`, and dependency version docs.
- Verified with `go test ./...` in `services/media-worker`.
- Notes for next session: implement Phase 6 FFmpeg/FFprobe processing and MinIO output upload, then decide Phase 7 media lifecycle event publishing.

### 2026-07-02

- Implemented `media-worker` Phase 1 and Phase 2 from `docs/development/media-worker-implementation-plan.md`.
- Replaced the placeholder server with config loading, structured JSON logging, request/correlation middleware, graceful shutdown, readiness checks, and Prometheus text metrics.
- Added domain models/state rules for processing jobs, attempts, inbox events and dead letters.
- Added PostgreSQL migration `001_processing_schema.sql`, repository interface, in-memory store, PostgreSQL store, idempotent job creation from uploaded events, job claim/lease, attempt start/success/failure transitions, and a skipped-by-default PostgreSQL integration harness using `MEDIA_WORKER_TEST_DATABASE_URL`.
- Added `github.com/jackc/pgx/v5 v5.8.0` to `services/media-worker` and updated dependency version docs.
- Updated `services/media-worker/README.md`, `docs/development/media-worker-implementation-plan.md`, and `docs/development/implementation-plan.md`.
- Verified with `go test ./...` in `services/media-worker`.
- Notes for next session: implement Phase 3 Kafka consumer for `video.uploaded.v1`, parse the video-service envelope, and create durable jobs through the new service/repository layer.

- Added `docs/development/media-worker-implementation-plan.md` as the focused roadmap/checklist for `media-worker`.
- Linked the new media-worker plan from `docs/development/README.md` and the high-level implementation plan.
- Notes for next session: this plan is now the source of truth for remaining media-worker phases.

- Finished the remaining `video-service` hardening pass before moving to `media-worker`.
- Added optional MinIO/S3 object metadata verification for upload confirmation through `VERIFY_UPLOAD_OBJECT=true`, including HEAD-based object checks, stable object-storage error codes, size/content-type mismatch handling, and object verification metrics.
- Added video status transition metrics and tests for worker-style internal status updates through `X-Internal-Token`.
- Updated `services/video-service/README.md` and `docs/development/video-service-implementation-plan.md`.
- Verified with `go test ./...` in `services/video-service`.
- Notes for next session: move to `media-worker` processing job persistence/consumer work; `video-service` still needs GitOps/Kubernetes manifests and smoke tests, but core service behavior is now enough for the next product flow.

### 2026-07-01

- Added the next `video-service` production slice: MinIO/S3-compatible presigned PUT URL generation, `Idempotency-Key` handling for upload request creation, owner/admin/internal authorization, and internal-token protected status updates.
- Added Redpanda/Kafka outbox publishing using `github.com/segmentio/kafka-go v0.4.51`; the worker polls publishable outbox rows, publishes full envelopes, marks events `published` after broker ack, marks publish failures as `failed`, and records outbox/DB metrics.
- Added migration `003_idempotency_outbox_attempts.sql`, repository support for idempotency lookup and outbox publish state, structured logs around upload/outbox workflows, and expanded Prometheus metrics for upload, presign, outbox and DB operations.
- Updated `services/video-service/README.md`, `docs/development/video-service-implementation-plan.md`, `docs/development/implementation-plan.md`, and dependency version docs.
- Verified with `go test ./...` in `services/video-service`.
- Notes for next session: define the `media-worker` processing contract and add tests for worker-driven internal status transitions; optional MinIO object metadata verification can follow.

- Wired `video-service` PostgreSQL integration tests into local compose and GitLab CI.
- Added a `postgres-test` compose profile, `make test-video-integration`, and `validate:video-postgres` CI job using `postgres:16-alpine`.
- Updated `services/video-service/README.md`, `deploy/docker-compose/README.md`, and implementation plan docs with the new test workflow.
- Verified with `go test ./...` in `services/video-service`, `docker compose config`, `docker compose --profile test config`, and `git diff --check`.
- Could not run `make test-video-integration` locally because Docker daemon was not running in the current environment.
- Notes for next session: implement the Redpanda/Kafka outbox publisher, then add MinIO presigned upload URL generation.

### 2026-06-30

- Added `video.uploaded.v1` outbox event creation for `services/video-service`.
- Added event payload builder under `internal/event`, an `OutboxEvent` domain model, outbox envelope migration, and repository support for writing the outbox event during upload confirmation.
- Updated service tests to assert confirm upload creates a pending outbox event with request/correlation evidence.
- Added a skipped-by-default PostgreSQL integration test harness for `PostgresStore` upload flow and outbox write using `VIDEO_SERVICE_TEST_DATABASE_URL`.
- Verified with `go test ./...` in `services/video-service` and `git diff --check`.
- Notes for next session: wire the PostgreSQL integration test into local compose or CI, then implement the Redpanda/Kafka publisher.

### 2026-06-29

- Added PostgreSQL persistence for `services/video-service`, including `migrations/001_video_schema.sql`, `DATABASE_URL` config validation, startup wiring, and `repository.PostgresStore`.
- Made upload confirmation atomic at the repository boundary by saving upload request, video status and status history in one store operation.
- Added config tests and updated `services/video-service/README.md`, `docs/development/video-service-implementation-plan.md`, `docs/development/implementation-plan.md`, and dependency version docs.
- Verified with `go test ./...` in `services/video-service`.
- Notes for next session: add database-backed integration tests, then write `video.uploaded.v1` to `outbox_events` during upload confirmation.

### 2026-06-28

- Added `docs/development/video-service-implementation-plan.md` as the focused roadmap/checklist for continuing `video-service`.
- Linked the service-specific plan from `docs/development/README.md` and referenced it from the broad implementation plan.
- Notes for next session: use the focused plan before adding PostgreSQL persistence, MinIO presigned uploads, outbox events or media-worker integration.

### 2026-06-28

- Added `docs/development/implementation-plan.md` as the status checklist for remaining product, DevSecOps, AIOps/RAG and evaluation milestones.
- Linked the implementation plan from `docs/development/README.md`.

### 2026-06-28

- Implemented the first production-shaped `services/video-service` slice.
- Added config, domain models/errors/state transitions, in-memory repository, video use cases, HTTP handlers, request/correlation ID middleware, metrics, readiness, and graceful server wiring.
- Added APIs for `POST /v1/videos/upload-requests`, `POST /v1/videos/{video_id}/uploaded`, `GET /v1/videos/{video_id}`, `GET /v1/videos`, and `PATCH /v1/videos/{video_id}/status`.
- Updated `services/video-service/README.md` to document current behavior and explicit remaining work.
- Verified with `go test ./...` in `services/video-service`.
- Notes for next session: Postgres persistence, MinIO presigned URL generation, outbox/event publishing, and idempotency are still pending.

### 2026-06-27

- Hardened `services/api-gateway` beyond the initial reverse-proxy skeleton.
- Added JWT access-token verification through identity JWKS using standard-library RS256 verification.
- Added auth middleware that enforces protected API prefixes, strips spoofed internal user headers, and forwards trusted `X-User-*` context after token verification.
- Replaced placeholder `/metrics` with basic Prometheus text counters and duration totals.
- Replaced placeholder `/readyz` with checks for route configuration and JWKS readiness when JWT verification is enabled.
- Updated `services/api-gateway/README.md` with implemented capabilities and configuration.
- Verified with `go test ./...` in `services/api-gateway`.

### 2026-06-24

- Added `PROJECT_PROGRESS.md` as the living handoff log for lost context and future AI sessions.
- Updated `AGENTS.md` so agents must read this progress file before coding.
- Added guidance that substantial work should update this file before the task is finished.

### 2026-06-24

- Added `PROJECT_CONTEXT.md` as a high-level project handoff file for context recovery.
- Linked `PROJECT_CONTEXT.md` from `README.md`, `docs/README.md`, and `AGENTS.md`.
- `PROJECT_CONTEXT.md` now summarizes thesis framing, mental model, repository map, service responsibilities, prioritized product flow, data ownership, storage rules, runtime baseline, Multi-Agent RAG model, current posture, and next steps.

### 2026-06-24

- Strengthened `AGENTS.md` so the default coding standard is production-oriented engineering rather than demo code.
- Added mandatory production-grade organization rules for Go product services and Python `aiops-service`.
- Added runtime/deployment rules covering config validation, health/readiness/metrics, structured logs, timeouts, and dependency failure handling.
- Updated the completion checklist to check deployability, service growth, observability, and failure paths.

### 2026-06-24

- Added `docs/development/product-code-rules.md` to define product implementation rules.
- Added the product rule link to `docs/development/README.md`.
- Updated `docs/architecture/repo-structure.md` with standard product service layout, AIOps service layout, and production-grade organization rules.

### 2026-06-23

- Discussed and refined the thesis direction.
- Agreed that the strongest framing is AIOps with Multi-Agent RAG for incident analysis/RCA in a Kubernetes microservices environment.
- Agreed that DevSecOps and GitOps are important as evidence sources and safe remediation boundaries.
- Agreed that the product app should be complete enough to generate realistic operational signals, but the thesis contribution remains the AIOps/RAG pipeline.

### 2026-06-23

- Added `AGENTS.md` as the first coding-agent rule file.
- The initial version established project purpose, thesis priority, service boundaries, data/storage rules, API/contract rules, Multi-Agent RAG rules, DevSecOps/GitOps rules, testing, documentation, dependency, and editing expectations.

## Next Useful Steps

Recommended engineering order:

1. Keep `identity-service` and `api-gateway` stable as the edge/auth foundation.
2. Add Redis-backed rate limiting to `api-gateway` when the edge/auth foundation needs another hardening pass.
3. Add richer media-worker observability metrics for queue lag, object storage and upstream status updates.
4. Add a local sample-video smoke test for `PROCESSING_MODE=ffmpeg`.
5. Add Kubernetes/GitOps manifests and smoke tests for `video-service` and `media-worker` when preparing deployment.
6. Implement a minimal ready-video feed in `feed-social-service`.
7. Add admin-facing views for users, videos, processing jobs, service health, incidents, and RCA reports.
8. Define incident fixtures, runbooks, ground truth, and evaluation metrics.
9. Implement `aiops-service` evidence schema, collectors, RAG pipeline, agents, RCA synthesis, and evaluator.

Recommended documentation order:

1. Keep `PROJECT_PROGRESS.md` current after major changes.
2. Keep `PROJECT_CONTEXT.md` stable and high-level.
3. Update architecture/API/development docs only when real behavior or rules change.
4. Add evaluation-specific docs once incident fixtures and baselines are clearer.

## Open Questions / Risks

- Scope risk: building too much product surface before the core incident/RCA loop works.
- Evaluation risk: Multi-Agent RAG needs clear baselines and measurable criteria, not only a demo.
- Implementation risk: remaining skeleton services need real config, persistence, APIs, events, tests, and observability before they feel production-shaped.
- Media-worker risk: scaffold, job persistence, Kafka consumption, placeholder/FFmpeg processors, video-service status updates, MinIO output upload and retry/dead-letter exist, but richer queue-lag/object-storage/upstream metrics, local sample-video smoke tests and deployment manifests are still pending.
- Video-service risk: upload idempotency, MinIO presigned URL, optional object metadata verification and Redpanda/Kafka outbox publishing exist, but richer outbox backoff controls, Kubernetes/GitOps manifests, smoke tests, and the `media-worker` processing contract are still pending.
- Gateway risk: Redis-backed rate limiting and richer route/upstream metrics are still pending.
- DevSecOps risk: security scans, CI, GitOps history, and deployment evidence must become real inputs to RCA, not decorative pipeline items.
- AIOps risk: agent outputs must cite evidence and expose uncertainty to avoid fluent but unsupported conclusions.

## Update Template

Use this format for future entries:

```text
### YYYY-MM-DD

- Added/changed/fixed ...
- Verified with ...
- Notes for next session ...
```
