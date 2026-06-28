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

As of 2026-06-28:

- The project direction is set: a Kubernetes-based microservices video/livestream platform used as a realistic testbed for AIOps RCA with Multi-Agent RAG, DevSecOps evidence, and GitOps-safe remediation.
- The thesis framing is centered on Multi-Agent RAG for incident investigation and root cause analysis, not on building a commercial video product for its own sake.
- Source code and product docs live in this repository.
- Kubernetes desired state is intended to live in the companion GitOps repository at `../aiops-gitops-manifests`.
- Product Go services have a consistent scaffold: `cmd/server`, `internal/config`, `internal/domain`, `internal/event`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, and `tests`.
- `identity-service` and `api-gateway` are more implemented than the other product services.
- `api-gateway` now has route proxying, request/correlation IDs, CORS, body limits, upstream timeout, JWT verification through identity JWKS, trusted user-context forwarding, internal header stripping, JSON gateway/auth errors, readiness checks, and basic Prometheus text metrics.
- `video-service` now has a production-shaped in-memory first implementation for upload requests, video metadata, upload confirmation, video status transitions, request/correlation IDs, readiness, metrics, and tests.
- Several product services beyond `identity-service`, `api-gateway`, and `video-service` are still mostly skeletons with health, readiness, and metrics placeholders.
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

## Work Log

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
3. Replace `video-service` in-memory store with PostgreSQL migrations/repository for videos, upload requests, status history and outbox events.
4. Add MinIO presigned upload URL generation and `video.uploaded.v1` outbox/event publishing.
5. Implement `media-worker` processing job persistence, retries, dead-letter state, and status updates.
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
- Video-service risk: current repository is in-memory only; Postgres, MinIO presigned upload and Redpanda/Kafka outbox publishing are still required for a real deployment flow.
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
