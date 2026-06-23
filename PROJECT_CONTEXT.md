# Project Context

This file is a quick handoff document for agents, contributors, or future sessions that need to understand the project after losing conversation context.

For coding rules, read `AGENTS.md` first. For current progress and handoff state, read `PROJECT_PROGRESS.md`. For product implementation rules, read `docs/development/product-code-rules.md`.

## One-Sentence Summary

This project builds a realistic Kubernetes-based microservices video/livestream platform and uses it as a testbed for an AIOps system based on Multi-Agent RAG, DevSecOps evidence, and GitOps-safe remediation.

## Thesis Framing

The proposed thesis topic is:

```text
Xay dung he thong AIOps tich hop Multi-Agent RAG phuc vu phan tich su co trong kien truc microservices tren Kubernetes
```

The academic contribution should not be framed as simply building a video app. The product platform is the operational environment that generates realistic incidents, logs, metrics, deployment changes, queue delays, worker failures, storage failures, and security or pipeline evidence.

The core contribution is:

- Multi-Agent RAG for incident investigation and root cause analysis.
- Evidence-aware RCA reports with citations to operational signals.
- DevSecOps and GitOps evidence as part of the investigation workflow.
- Safe remediation suggestions that do not directly mutate production state.

## Mental Model

Think of the project as two connected systems:

1. Product system: a microservices video/livestream platform.
2. AIOps system: a Multi-Agent RAG service that investigates incidents from the product system.

The product system must be realistic enough to create useful operational signals. It does not need to compete with YouTube, TikTok, or a full commercial live platform.

The AIOps system must be evidence-driven. It should collect evidence, reason over it, produce RCA reports, and make uncertainty visible.

## Repository Map

This source repo contains application code, docs, local development setup, tests, and supporting tools.

Important directories:

- `services/`: backend services, workers, and `aiops-service`.
- `apps/`: admin web and mobile app placeholders.
- `packages/`: shared contracts, proto files, and shared documentation.
- `deploy/`: local compose, Kubernetes templates, Helm templates, scripts.
- `infra/`: platform notes for Kubernetes, ingress, observability, registry, storage.
- `docs/`: architecture, API design, development rules, incidents, runbooks, experiments, thesis material.
- `tests/`: smoke, e2e, and load test areas.
- `tools/`: incident injector, log generator, RCA evaluator.

The companion GitOps repo is:

```text
../aiops-gitops-manifests
```

That repo owns Kubernetes desired state and Argo CD sync configuration. Deployment-related decisions should preserve the GitOps model.

## Current Service Intent

`api-gateway`

- Public HTTP entrypoint under `/api/v1/*`.
- Owns routing, request context, policy, body limit, CORS, timeout, observability.
- Must not own product business logic or product data.

`identity-service`

- Owns users, credentials, sessions, profile, JWT, OAuth, auth audit logs.
- PostgreSQL is canonical outside local-only fallback flows.
- Redis is for auth rate limiting.

`video-service`

- Owns video metadata, upload requests, object keys, and video lifecycle state.
- Emits video lifecycle events.
- Must not run FFmpeg.

`media-worker`

- Owns processing jobs, attempts, retries, dead-letter behavior, worker state.
- Consumes video-uploaded events and updates video state through controlled APIs or events.
- Important incident surface for queue lag, FFmpeg errors, OOM, retry storms, and worker failures.

`feed-social-service`

- Owns feed view, likes, comments, follows, and optional denormalized feed items.
- Should stay simple until the thesis requires more.

`live-service`

- Owns live sessions, stream keys, live state, and MediaMTX integration.
- Build only what is needed for realistic demo and incident signals.

`aiops-service`

- Thesis core.
- Owns incidents, evidence items, evidence packs, RCA reports, agent runs, and RAG flow.
- Collects evidence from Kubernetes, Loki, Prometheus, Argo CD, GitLab, Harbor/Trivy, runbooks, and related sources.
- Must not directly apply changes to Kubernetes or product databases.

## Standard Code Organization

Go product services should follow this shape:

```text
cmd/server/             process entrypoint and dependency wiring
internal/config/        environment loading and validation
internal/domain/        domain models, states, invariants, domain errors
internal/handler/       HTTP transport and request/response mapping
internal/service/       use cases and business workflows
internal/repository/    persistence interfaces and implementations
internal/event/         event contracts, producers, consumers, outbox helpers
internal/observability/ logging, metrics, middleware, readiness helpers
migrations/             service-owned schema migrations
tests/                  integration, smoke, or service-level test assets
```

Do not put production behavior directly in `main.go` beyond wiring and startup. Keep handlers thin, services focused on use cases, repositories focused on persistence, and domain packages responsible for states and invariants.

Python AIOps code should preserve these boundaries:

```text
app/api/          HTTP API surface
app/core/         configuration and runtime concerns
app/collectors/   external evidence collectors
app/agents/       planner and specialist agent logic
app/rag/          chunking, embedding, retrieval, vector-store integration
app/redaction/    secret and sensitive-data filtering
app/scoring/      confidence, ranking, evaluation support
app/schemas/      request, response, evidence and RCA schemas
```

## Product Flow To Prioritize

The highest-value end-to-end product flow is:

```text
user registers/logs in
-> creator creates upload request
-> video metadata and object key are stored
-> raw file lands in MinIO
-> video.uploaded event is emitted
-> media-worker creates processing job
-> worker processes file or placeholder
-> video becomes ready or failed
-> feed exposes ready videos
-> admin observes video/job/service state
-> incident is created or detected
-> aiops-service builds evidence pack and RCA report
```

This flow should be made reliable before expanding into richer livestream, recommendation, mobile, or social features.

## Data Ownership

Do not let services read each other's databases.

Canonical ownership:

- User/session/auth data: `identity-service`.
- Video metadata and upload lifecycle: `video-service`.
- Processing jobs and attempts: `media-worker`.
- Feed, likes, comments, follows: `feed-social-service`.
- Live sessions and stream keys: `live-service`.
- Incidents, evidence, RCA reports, agent runs: `aiops-service`.

Use APIs for synchronous reads and events for asynchronous propagation.

## Storage Rules

- PostgreSQL: transactional and lifecycle source of truth.
- MongoDB: flexible documents such as incidents, evidence, RCA reports, comments, feed read models.
- Redis: cache, rate limit, locks, counters, idempotency, short-lived state only.
- MinIO: raw and processed media objects.
- Redpanda/Kafka-compatible broker: event flow.
- Qdrant: vector store for RAG.

Redis must never be the only copy of important business data.

## Runtime Baseline

- Go services use Go `1.24` with toolchain `go1.24.13`.
- `aiops-service` uses Python `3.12.13`.
- Dependency policy is defined in `docs/development/dependency-versioning.md`.

Do not add floating dependency versions in production manifests.

## Multi-Agent RAG Model

Suggested agent responsibilities:

- Planner Agent: decides investigation plan and evidence needs.
- Log Agent: analyzes Loki/service logs.
- Metric Agent: analyzes Prometheus metrics.
- Deployment Agent: checks GitOps, CI/CD, image, config, and release history.
- Kubernetes Agent: checks pod, event, node, probe, restart, scheduling, and resource state.
- Runbook/Retrieval Agent: retrieves relevant runbooks, incident records, and operational docs.
- Evidence Validation Agent: checks whether conclusions are supported by evidence.
- RCA Synthesis Agent: produces final RCA report with confidence and evidence references.
- Remediation Suggestion Agent: proposes safe actions such as rollback, config fix, scaling, or follow-up checks.
- Security/DevSecOps Agent: checks scan results, vulnerabilities, secret issues, policy violations, and pipeline anomalies.

Agent boundaries can evolve, but every conclusion must be grounded in evidence.

## What To Avoid

- Do not grow the video product into a full commercial platform before the core incident/RCA loop works.
- Do not bypass `api-gateway` for public client flows.
- Do not put business logic into `api-gateway`.
- Do not use direct database reads across service boundaries.
- Do not store secrets, raw tokens, presigned URLs, or stream keys in logs or evidence.
- Do not implement automatic production mutation as the default remediation path.
- Do not add new services unless there is a clear boundary and thesis value.

## Current Development Posture

The repository is intentionally scaffolded for serious growth:

- Product Go services use a consistent `cmd/server` and `internal/*` structure.
- `identity-service` and `api-gateway` are more implemented than other product services.
- Several services still have placeholder health/readiness/metrics behavior.
- The next serious product milestone should make one complete video-processing flow work end to end.
- The next serious thesis milestone should define incident datasets, evidence schemas, agent outputs, baselines, and evaluation metrics.

When resuming work, first inspect the real code and git status. This file gives orientation, not a guarantee that implementation has caught up with the intended design.

## Best Next Steps

Recommended order:

1. Harden `identity-service` and `api-gateway` as the stable edge/auth foundation.
2. Implement `video-service` upload request, metadata, lifecycle state, and event emission.
3. Implement `media-worker` processing jobs, retries, state transitions, and failure evidence.
4. Implement minimal `feed-social-service` feed of ready videos.
5. Add admin views for users, videos, jobs, health, incidents, and RCA reports.
6. Build incident fixtures and runbooks.
7. Implement AIOps collectors and evidence schema.
8. Implement Multi-Agent RAG RCA pipeline and evaluator.

Keep each step production-shaped: config, validation, tests, observability, docs, and clear failure paths.

## Docs To Read Next

Start here:

1. `AGENTS.md`
2. `PROJECT_PROGRESS.md`
3. `README.md`
4. `docs/architecture/product-design.md`
5. `docs/architecture/service-boundaries.md`
6. `docs/architecture/data-ownership.md`
7. `docs/architecture/database-design.md`
8. `docs/architecture/repo-structure.md`
9. `docs/development/product-code-rules.md`
10. `docs/api/rest-api-design.md`
11. `packages/contracts/event-contracts.md`

For service-specific work, also read the target service README and any plan under `docs/service/`.
