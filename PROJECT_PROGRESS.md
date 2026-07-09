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

As of 2026-07-07:

- The project direction is set: a Kubernetes-based microservices video/livestream platform used as a realistic testbed for AIOps RCA with Multi-Agent RAG, DevSecOps evidence, and GitOps-safe remediation.
- The thesis framing is centered on Multi-Agent RAG for incident investigation and root cause analysis, not on building a commercial video product for its own sake.
- Source code and product docs live in this repository.
- Kubernetes desired state is intended to live in the companion GitOps repository at `../aiops-gitops-manifests`.
- Product Go services have a consistent scaffold: `cmd/server`, `internal/config`, `internal/domain`, `internal/event`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, and `tests`.
- `identity-service` and `api-gateway` are more implemented than the other product services.
- `api-gateway` now has route proxying, request/correlation IDs, CORS, body limits, upstream timeout, JWT verification through identity JWKS, trusted user-context forwarding, internal header stripping, JSON gateway/auth errors, readiness checks, and basic Prometheus text metrics.
- `video-service` now has a production-shaped implementation for upload requests, video metadata, upload confirmation with optional MinIO/S3 object metadata verification, video status transitions, request/correlation IDs, readiness, metrics, tests, PostgreSQL persistence with local in-memory fallback, local/CI DB integration workflow, idempotent upload intent creation, MinIO/S3 presigned upload URLs, owner/internal authorization, pending outbox writes for `video.uploaded.v1`, and a Redpanda/Kafka outbox publisher worker.
- `media-worker` now has a production-shaped scaffold, config validation, health/readiness/metrics, domain models for processing jobs/attempts/dead letters, PostgreSQL schema and repository, local in-memory test store, Kafka consumer for `video.uploaded.v1`, placeholder processing runner, FFmpeg/FFprobe processing mode, MinIO raw download/output upload, thumbnail generation, video-service internal status update client, retry/backoff, dead-letter behavior, lifecycle event contract builders, richer operational metrics/logging, a PostgreSQL integration-test target, and an FFmpeg smoke test.
- `feed-social-service` now has a production-shaped scaffold, config validation, health/readiness/metrics, PostgreSQL feed read model foundation, local in-memory fallback, `feed_items`, `video_social_counters`, `inbox_events`, idempotent ready-video upsert, stable feed list repository query, `GET /v1/feed`, `video.ready.v1` Kafka/Redpanda consumer, controlled internal ingestion fallback, idempotent likes, PostgreSQL comments MVP, durable like/comment counters, follows, optional Redis cache for guest feed/social counters, repository/API/event/cache tests, and a skipped-by-default PostgreSQL integration harness. It still needs full compose/Kubernetes wiring.
- `live-service` now has a production-shaped MVP for basic app APIs: config validation, local in-memory fallback, PostgreSQL schema/repository, create/list/get live sessions, owner/admin start/end lifecycle transitions, stream key hash storage, live event audit rows, readiness, metrics, and gateway routing for `/api/v1/live-sessions`.
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

### 2026-07-09

- Applied a security hardening pass before frontend work.
- Local compose now binds internal services and dependencies to `127.0.0.1`, keeps only `api-gateway` configurable for external binding, and requires local Postgres, MinIO and internal-token secrets instead of using committed defaults.
- Updated `.env.example` and compose docs to require local secrets and explain the gateway-only access model.
- `identity-service` no longer trusts `X-Forwarded-For`/`X-Real-IP` by default; added `TRUST_PROXY_HEADERS` for trusted proxy deployments.
- `api-gateway` now sanitizes forwarded IP/proto headers before proxying so trusted-proxy deployments do not accept client-spoofed forwarding headers.
- Added Google OAuth redirect allowlist support through `GOOGLE_ALLOWED_REDIRECT_URIS`, required outside local/dev/test when Google OAuth is enabled.
- `feed-social-service` now requires `INTERNAL_API_TOKEN` outside local/dev/test because the internal ingestion route is registered.
- Updated the product smoke test to use the real upload -> ready event -> feed ingestion path instead of direct internal feed seeding.
- Verified with `go test ./...` across Go services, `POSTGRES_PASSWORD=test-postgres MINIO_ROOT_USER=test-minio MINIO_ROOT_PASSWORD=test-minio-pass INTERNAL_API_TOKEN=test-internal-token docker compose config`, and `make smoke-product`.

- Fixed the ready-video event handoff for the product feed flow.
- `media-worker` now sends processed object key, thumbnail key, duration and size metadata when updating a video to `ready`.
- `video-service` now persists public `ready` status transitions and `video.ready.v1` outbox events atomically, using the canonical video metadata required by `feed-social-service`.
- Updated event contract and service docs to clarify that `video-service` owns canonical lifecycle publishing while `media-worker` supplies processing asset metadata.
- Verified with `go test ./...` in `services/video-service` and `services/media-worker`, `make smoke-product`, and a local compose upload-to-feed smoke path without internal feed seeding.
- Notes for next session: consider replacing the current smoke script's internal feed seed with the true ready-event path now that the event handoff is implemented.

### 2026-07-07

- Added production-shaped local Docker Compose wiring for the core product stack.
- Compose now builds/runs `api-gateway`, `identity-service`, `video-service`, `media-worker`, `feed-social-service`, and `live-service` with PostgreSQL, Redis, MinIO, Redpanda, MediaMTX, and Qdrant dependencies.
- Added a PostgreSQL init script for per-service local databases, a migration job for service-owned schemas, and a MinIO bucket init job for raw/processed/thumbnail buckets.
- Optimized service Dockerfiles by copying `go.sum` into Go build dependency layers where available, keeping `media-worker` on an FFmpeg-capable Alpine runtime, and running `aiops-service` as a non-root user.
- Added Makefile helpers for compose validation, startup, shutdown, logs, and AIOps profile startup.
- Verified the compose stack with Docker running: product services build, migrations apply, MinIO buckets initialize, gateway and service readiness endpoints return healthy, and gateway smoke paths for auth, live create/start/end, upload request, feed, like and social counters work.
- Fixed a PostgreSQL query bug in `feed-social-service` where unqualified feed item columns became ambiguous after joining social counters.
- Notes for next session: add a repeatable product smoke test script/Makefile target for auth, upload intent, feed/social APIs, and live create/start/end through `api-gateway`.

- Implemented the first production-shaped `live-service` slice for basic app APIs.
- Replaced the placeholder server with config loading, JSON structured logs, body limit, request/correlation middleware, graceful shutdown, readiness through store ping, and Prometheus text metrics.
- Added live session domain/state rules, PostgreSQL migration `001_live_schema.sql`, local in-memory store, PostgreSQL repository, and DB operation instrumentation.
- Added `POST /v1/live-sessions`, `GET /v1/live-sessions`, `GET /v1/live-sessions/{live_session_id}`, `POST /v1/live-sessions/{live_session_id}/start`, and `POST /v1/live-sessions/{live_session_id}/end`.
- Added trusted user-context authorization for owner/admin lifecycle actions and ensured plaintext `stream_key` is returned only during session creation while the database stores a hash.
- Fixed `api-gateway` live route matching so `POST /api/v1/live-sessions` routes correctly without requiring a trailing slash, and added nested social routing so video like/comment/social plus user follow endpoints reach `feed-social-service` through public API paths.
- Notes for next session: add compose/Kubernetes wiring and a gateway-level smoke test that covers auth -> live create/start/end plus video/feed basics.

- Implemented `feed-social-service` Phase 7 and Phase 8 from `docs/development/feed-social-service-implementation-plan.md`.
- Added migration `003_follows_schema.sql`, follow domain status, idempotent `PUT /v1/users/{user_id}/follow`, idempotent `DELETE /v1/users/{user_id}/follow`, self-follow rejection, trusted user context enforcement, and handler/service/repository tests.
- Added optional Redis cache through `github.com/redis/go-redis/v9 v9.20.1`, `CACHE_ENABLED`, `REDIS_URL`, and `FEED_CACHE_TTL`.
- Added cache abstraction with no-op local implementation and Redis implementation for guest feed pages and social counters.
- Wired cache read-through/fail-open behavior into feed/social reads and invalidation after ready-video ingestion, likes, comments and deletes.
- Added cache metrics for hit, miss, error, success and operation duration.
- Updated `services/feed-social-service/README.md`, `docs/development/feed-social-service-implementation-plan.md`, `docs/development/implementation-plan.md`, and dependency version docs.
- Verified with `go test ./...` in `services/feed-social-service`.
- Notes for next session: add a local compose smoke test for ready video ingestion -> feed listing -> like/comment/follow, then decide whether to move to `live-service`.

- Implemented `feed-social-service` Phase 5 and Phase 6 from `docs/development/feed-social-service-implementation-plan.md`.
- Added migration `002_social_schema.sql` for `likes` and PostgreSQL MVP `comments`.
- Added domain models/validation for likes and comments, including visible/hidden/deleted/blocked comment statuses and public body redaction for deleted/non-visible comments.
- Added idempotent `PUT /v1/videos/{video_id}/like`, `DELETE /v1/videos/{video_id}/like`, `GET /v1/videos/{video_id}/social`, `POST /v1/videos/{video_id}/comments`, `GET /v1/videos/{video_id}/comments`, and `DELETE /v1/comments/{comment_id}`.
- Added transactional PostgreSQL updates for `like_count` and `comment_count`, plus local in-memory implementations for tests/dev.
- Added handler, service, repository and domain tests for like idempotency, trusted user context, comment create/list/delete, body validation and counter updates.
- Updated `services/feed-social-service/README.md`, `docs/development/feed-social-service-implementation-plan.md`, and `docs/development/implementation-plan.md`.
- Verified with `go test ./...` in `services/feed-social-service`.
- Notes for next session: add follows if needed for the product demo, or move to a compose smoke test that covers ready video ingestion, feed listing, like and comment.

### 2026-07-06

- Implemented `feed-social-service` Phase 3 and Phase 4 from `docs/development/feed-social-service-implementation-plan.md`.
- Added `GET /v1/feed` with limit parsing, max-limit cap, cursor pagination, response envelope, active-feed filtering through the repository layer, request ID propagation, and handler tests for empty/populated/cursor/limit behavior.
- Added controlled fallback ingestion `POST /v1/internal/feed-items` guarded by `X-Internal-Token` for local/dev/MVP seeding without reading the `video-service` database.
- Added `video.ready.v1` event parsing and a Kafka/Redpanda consumer worker using `github.com/segmentio/kafka-go v0.4.51`; offsets are committed only after durable feed upsert, while invalid events are committed after being recorded.
- Added config validation for `CONSUMER_ENABLED`, `KAFKA_BROKERS`, `VIDEO_EVENTS_TOPIC`, `CONSUMER_GROUP`, and `INTERNAL_API_TOKEN`.
- Added feed result count and event age metrics, plus event parser/consumer tests.
- Updated `services/feed-social-service/README.md`, `docs/development/feed-social-service-implementation-plan.md`, `docs/development/implementation-plan.md`, and dependency version docs.
- Verified with `go test ./...` in `services/feed-social-service`.
- Notes for next session: implement Phase 5 likes and durable counters, then decide whether comments/follows are needed before moving to `live-service`.

### 2026-07-05

- Implemented `feed-social-service` Phase 1 and Phase 2 from `docs/development/feed-social-service-implementation-plan.md`.
- Replaced the placeholder server with config loading, JSON structured logs, body limit, request/correlation middleware, graceful shutdown, readiness through store ping, metrics middleware, PostgreSQL wiring, and explicit local in-memory fallback.
- Added feed domain models/errors for feed items, counters and inbox idempotency; added PostgreSQL migration `001_feed_schema.sql` for `feed_items`, `video_social_counters`, and `inbox_events`.
- Added repository interfaces plus in-memory, PostgreSQL and instrumented stores with idempotent ready-video upsert and stable feed listing by `ready_at DESC, video_id DESC`.
- Added `github.com/jackc/pgx/v5 v5.8.0` to `services/feed-social-service` and updated dependency/version docs.
- Updated `services/feed-social-service/README.md`, `docs/development/feed-social-service-implementation-plan.md`, and `docs/development/implementation-plan.md`.
- Verified with `go test ./...` in `services/feed-social-service`.
- Notes for next session: implement Phase 3 `GET /v1/feed`, then choose Phase 4 ingestion path for `video.ready.v1` or a controlled internal MVP ingestion route.

### 2026-07-04

- Added `docs/development/feed-social-service-implementation-plan.md` as the focused roadmap/checklist for building the ready-video feed and basic social service.
- Linked the feed-social plan from `docs/development/README.md` and the high-level implementation plan.
- Notes for next session: start `feed-social-service` Phase 1 scaffold, then PostgreSQL feed read model and `GET /v1/feed`.

- Implemented `media-worker` Phase 8 observability work from `docs/development/media-worker-implementation-plan.md`.
- Added job status gauges, runnable queue depth/oldest-age gauges, attempt outcome/error-code counters, observed `video.uploaded.v1` event age counters, database operation metrics, MinIO dependency metrics, and video-service status update dependency metrics.
- Added repository `Stats` support for in-memory and PostgreSQL stores plus a store instrumentation wrapper.
- Added MinIO object-store and video-service status-client instrumentation wrappers.
- Expanded worker/consumer logs with service, environment, worker, job, attempt, video, request, correlation and error-code context.
- Added `make test-media`, `make test-media-integration`, `make smoke-media-ffmpeg`, a `postgres-media-test` compose service, and a smoke-tag FFmpeg processor test.
- Verified with `go test ./...` in `services/media-worker`, `docker compose --profile test config`, `make -n test-media-integration`, `make -n smoke-media-ffmpeg`, and `make smoke-media-ffmpeg`.
- Could not run `make test-media-integration` because the local Docker daemon was not running.
- Notes for next session: add Kubernetes/GitOps manifests and resource sizing for `video-service`/`media-worker`, then add a full compose smoke test from upload event to processing status update.

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
3. Add Kubernetes/GitOps manifests and resource requests/limits for `video-service` and `media-worker`.
4. Add a full compose smoke test for the video upload-to-processing flow.
5. Add local compose smoke test for ready feed plus like/comment/follow, then move to `live-service` if the product flow is enough.
6. Add admin-facing views for users, videos, processing jobs, service health, incidents, and RCA reports.
7. Define incident fixtures, runbooks, ground truth, and evaluation metrics.
8. Implement `aiops-service` evidence schema, collectors, RAG pipeline, agents, RCA synthesis, and evaluator.

Recommended documentation order:

1. Keep `PROJECT_PROGRESS.md` current after major changes.
2. Keep `PROJECT_CONTEXT.md` stable and high-level.
3. Update architecture/API/development docs only when real behavior or rules change.
4. Add evaluation-specific docs once incident fixtures and baselines are clearer.

## Open Questions / Risks

- Scope risk: building too much product surface before the core incident/RCA loop works.
- Evaluation risk: Multi-Agent RAG needs clear baselines and measurable criteria, not only a demo.
- Implementation risk: remaining skeleton services need real config, persistence, APIs, events, tests, and observability before they feel production-shaped.
- Media-worker risk: scaffold, job persistence, Kafka consumption, placeholder/FFmpeg processors, video-service status updates, MinIO output upload, retry/dead-letter, richer metrics/logs and FFmpeg smoke tests exist, but Kubernetes/GitOps manifests, resource sizing and full upload-to-processing smoke tests are still pending.
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
