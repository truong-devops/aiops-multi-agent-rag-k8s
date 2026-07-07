# Implementation Plan

Tai lieu nay theo doi cac moc can lam de dua du an tu scaffold thanh mot he thong co the demo, van hanh va phuc vu khoa luan. Day la checklist song; cap nhat file nay khi mot moc lon thay doi trang thai.

Legend:

- `[x]` Done
- `[~]` In progress / partial
- `[ ]` Not started

## Current Snapshot

As of 2026-07-07:

- `identity-service`: da co auth/profile/JWT/OAuth/JWKS/PostgreSQL/Redis-facing design o muc tot hon cac service khac.
- `api-gateway`: da co routing, request/correlation ID, JWT verify qua JWKS, trusted user-context forwarding, readiness va basic metrics.
- `video-service`: da co in-memory local mode, PostgreSQL persistence, local/CI DB integration test workflow, upload idempotency, MinIO/S3 presigned upload URL, optional object metadata verification, owner/internal authorization va Redpanda/Kafka outbox publisher cho `video.uploaded.v1`.
- `media-worker`: da co production-shaped scaffold, PostgreSQL job persistence, `video.uploaded.v1` consumer, placeholder runner, video-service status update client, retry/backoff, dead-letter, FFmpeg/FFprobe processing mode, MinIO raw download, processed output upload, thumbnail upload, lifecycle event contract builders, richer operational metrics/logging, PostgreSQL integration test target va FFmpeg smoke test.
- `feed-social-service`: da co production-shaped scaffold, config validation, health/readiness/metrics, PostgreSQL feed read model foundation, in-memory local store, `feed_items`, `video_social_counters`, `inbox_events`, idempotent ready-video upsert, `GET /v1/feed`, Kafka/Redpanda consumer cho `video.ready.v1`, controlled internal ingestion fallback, idempotent likes, PostgreSQL comments MVP, durable like/comment counters, repository/API/event tests; chua co follows/cache.
- `live-service`: van chu yeu la skeleton.
- `aiops-service`: da co package layout, chua co RCA pipeline that.

## Milestone 0: Project Rules And Handoff

- `[x]` Add `AGENTS.md` as coding-agent rule file.
- `[x]` Add `PROJECT_CONTEXT.md` for context recovery.
- `[x]` Add `PROJECT_PROGRESS.md` for living handoff/progress log.
- `[x]` Add product code rules in `docs/development/product-code-rules.md`.
- `[x]` Document standard service layout in `docs/architecture/repo-structure.md`.
- `[~]` Keep `PROJECT_PROGRESS.md` updated after substantial work.

Done criteria:

- New AI session can read `AGENTS.md`, `PROJECT_CONTEXT.md`, `PROJECT_PROGRESS.md` and understand direction, progress, and next work.

## Milestone 1: Edge And Identity Foundation

- `[x]` Implement `api-gateway` route proxying for product services.
- `[x]` Propagate `X-Request-ID` and `X-Correlation-ID`.
- `[x]` Add CORS, body limit and upstream timeout in gateway.
- `[x]` Add JWT verification via identity JWKS.
- `[x]` Strip spoofed internal user headers and forward trusted `X-User-*` context.
- `[x]` Add gateway readiness and basic Prometheus text metrics.
- `[ ]` Add Redis-backed gateway rate limiting.
- `[ ]` Add route/upstream-level gateway metrics.
- `[ ]` Add richer access logs with upstream service, route prefix and auth failure reason.
- `[ ]` Add optional OpenTelemetry trace propagation.

Done criteria:

- Client traffic enters through gateway.
- Protected product routes receive trusted user context.
- Gateway creates useful logs/metrics for RCA.
- Gateway can fail clearly when identity/JWKS is unavailable.

## Milestone 2: Video Upload Core

Detailed service checklist: `docs/development/video-service-implementation-plan.md`.

- `[x]` Add `video-service` config, domain models, errors and state transitions.
- `[x]` Add in-memory repository for local/test execution.
- `[x]` Add upload request API: `POST /v1/videos/upload-requests`.
- `[x]` Add upload confirmation API: `POST /v1/videos/{video_id}/uploaded`.
- `[x]` Add get/list video APIs.
- `[x]` Add status update API: `PATCH /v1/videos/{video_id}/status`.
- `[x]` Add request/correlation ID propagation, readiness and metrics.
- `[x]` Add handler/service tests for first upload flow.
- `[x]` Add PostgreSQL migration for `videos`, `upload_requests`, `video_assets`, `video_status_history`, `outbox_events`.
- `[x]` Add PostgreSQL repository implementation.
- `[x]` Add idempotency handling for upload request creation.
- `[x]` Add MinIO presigned upload URL generation.
- `[x]` Add outbox write for `video.uploaded.v1`.
- `[x]` Add event publisher worker or publish path for Redpanda/Kafka.
- `[x]` Add integration tests for database-backed flow.

Done criteria:

- A creator can create an upload request through gateway.
- Video metadata is persisted durably.
- Upload confirmation commits video state and produces outbox evidence.
- `video.uploaded.v1` can be consumed by `media-worker`.

## Milestone 3: Media Worker Processing

Detailed service checklist: `docs/development/media-worker-implementation-plan.md`.

- `[x]` Define processing job domain and state machine.
- `[x]` Add PostgreSQL migration for `processing_jobs`, `processing_attempts`, `dead_letters`.
- `[x]` Consume `video.uploaded.v1`.
- `[x]` Create processing job and first attempt.
- `[x]` Add processing placeholder path before real FFmpeg.
- `[x]` Add retry and dead-letter behavior.
- `[x]` Update video status through `video-service` API or controlled event.
- `[x]` Add FFmpeg/FFprobe processing and MinIO processed/thumbnail upload.
- `[~]` Emit `video.processing_started.v1`, `video.ready.v1`, `video.processing_failed.v1`.
- `[x]` Add metrics for job count, success, failure, retry, dead-letter, queue lag.
- `[x]` Add incident-friendly logs with `video_id`, `job_id`, `attempt`, `error_code`.

Done criteria:

- Uploaded video creates a processing job.
- Worker can mark videos ready or failed.
- Failure paths create durable evidence for RCA.

## Milestone 4: Feed And Basic Social

Detailed service checklist: `docs/development/feed-social-service-implementation-plan.md`.

- `[x]` Add feed-social production-shaped scaffold.
- `[x]` Add PostgreSQL feed read model foundation.
- `[x]` Implement minimal feed API for ready videos.
- `[x]` Consume `video.ready.v1` or query via controlled API for MVP.
- `[x]` Add likes.
- `[x]` Add comments.
- `[ ]` Add follows.
- `[ ]` Add cache only after durable source of truth is clear.
- `[ ]` Add tests for feed visibility and ready-only behavior.

Done criteria:

- Viewer can list ready public videos.
- Feed does not depend on direct reads from `video-service` database.

## Milestone 5: Admin / Operations Surface

- `[ ]` Scaffold or implement admin web.
- `[ ]` Show users/auth status.
- `[ ]` Show videos and upload state.
- `[ ]` Show processing jobs and failures.
- `[ ]` Show service health/readiness.
- `[ ]` Show incidents and RCA reports.
- `[ ]` Ensure admin web calls APIs through `api-gateway`.

Done criteria:

- Demo operator can observe the product flow and incident/RCA flow from one UI.

## Milestone 6: DevSecOps And GitOps Evidence

- `[~]` Keep source repo and GitOps repo separated.
- `[ ]` Add or finalize CI validation for Go services.
- `[ ]` Add image build/publish flow.
- `[ ]` Add secret scan.
- `[ ]` Add dependency/vulnerability scan.
- `[ ]` Add SBOM generation.
- `[ ]` Add GitOps image/tag update workflow.
- `[ ]` Add Argo CD sync evidence collector target.
- `[ ]` Store deployment history as evidence for RCA.

Done criteria:

- Deploy changes are auditable.
- AIOps can use pipeline/deployment evidence when analyzing incidents.

## Milestone 7: AIOps Evidence Model

- `[ ]` Define incident schema.
- `[ ]` Define evidence item schema.
- `[ ]` Define RCA report schema.
- `[ ]` Define agent run schema.
- `[ ]` Add MongoDB persistence for incidents/evidence/RCA.
- `[ ]` Add redaction rules for secrets, tokens, presigned URLs and sensitive headers.
- `[ ]` Add runbook document format.
- `[ ]` Add Qdrant collection conventions for embeddings.

Done criteria:

- Incidents, evidence, agent outputs and RCA reports have stable schemas.
- Evidence can be referenced from final RCA output.

## Milestone 8: Collectors

- `[ ]` Kubernetes collector for pods, events, restarts, probes and resource state.
- `[ ]` Loki collector for service logs.
- `[ ]` Prometheus collector for metrics.
- `[ ]` Argo CD collector for deployment/sync state.
- `[ ]` GitLab collector for pipeline/job history if used.
- `[ ]` Harbor/Trivy collector for image/security evidence if used.
- `[ ]` Runbook collector/retriever.
- `[ ]` Collector time-window controls around incident time.

Done criteria:

- Given an incident, AIOps can build an evidence pack from multiple operational sources.

## Milestone 9: Multi-Agent RAG RCA

- `[ ]` Planner Agent.
- `[ ]` Log Agent.
- `[ ]` Metric Agent.
- `[ ]` Deployment Agent.
- `[ ]` Kubernetes Agent.
- `[ ]` Runbook/Retrieval Agent.
- `[ ]` Evidence Validation Agent.
- `[ ]` RCA Synthesis Agent.
- `[ ]` Remediation Suggestion Agent.
- `[ ]` Security/DevSecOps Agent.
- `[ ]` Confidence scoring and uncertainty output.
- `[ ]` Evidence citations in final RCA.

Done criteria:

- RCA report includes root cause hypothesis, evidence references, confidence, impact, timeline and safe remediation.
- Agent conclusions are grounded in evidence, not only free-form reasoning.

## Milestone 10: Incident Dataset And Evaluation

- `[ ]` Define incident scenarios.
- `[ ]` Add incident injection scripts.
- `[ ]` Add ground-truth RCA for each incident.
- `[ ]` Add baseline single-agent or non-agent RAG comparison.
- `[ ]` Add metrics: root-cause accuracy, evidence coverage, hallucination rate, time-to-diagnosis, remediation safety.
- `[ ]` Add RCA evaluator tool workflow.
- `[ ]` Record experiment results in `docs/experiments`.

Done criteria:

- Thesis can compare the proposed Multi-Agent RAG approach against a baseline with repeatable incidents.

## Suggested Immediate Next Steps

1. Add GitOps/Kubernetes manifests and resource requests/limits for `video-service` and `media-worker`.
2. Add full compose smoke test for the video upload-to-processing flow.
3. Keep gateway rate limiting as a hardening task after video flow has durable storage.
4. Implement `feed-social-service` Phase 7 follows if needed, then add a local compose smoke test for ready feed plus like/comment.

## Update Rule

When a major task is completed:

- Update the relevant checklist item from `[ ]` to `[~]` or `[x]`.
- Add new work to the right milestone instead of hiding it in notes.
- Keep `PROJECT_PROGRESS.md` for narrative handoff and this file for checklist status.
