# AGENTS.md

This file defines the working rules for coding agents contributing to this repository.

When a user asks an agent to code in this repo, the agent must read this file first and follow it unless the user explicitly asks to do otherwise.

The default standard for code in this repository is production-oriented engineering. Do not treat work as throwaway demo code. Even when a feature is small, organize it so it can grow, be tested, be observed, and be deployed to a real Kubernetes environment.

## 1. Project Purpose

This repository is a thesis-oriented engineering project.

The system has two connected goals:

1. Build a realistic microservices-based video/livestream platform on Kubernetes.
2. Use that platform as an operational testbed for an AIOps system based on Multi-Agent RAG, DevSecOps, and GitOps evidence.

The product exists to generate realistic signals such as logs, metrics, deployment history, queue lag, storage failures, worker failures, and configuration drift.

The primary thesis focus is:

- Multi-Agent RAG for incident investigation and root cause analysis.
- DevSecOps and GitOps as evidence sources and safe remediation boundaries.

Agents must preserve this priority. Product work is important, but the app is not the final academic contribution by itself.

## 2. Execution Priority

When choosing between multiple useful changes, use this order of priority:

1. Build production-shaped code: clear boundaries, explicit contracts, testable logic, observable runtime behavior, and deployable configuration.
2. Protect the thesis focus: incident analysis, evidence collection, RCA quality, evaluation readiness.
3. Keep the core product flow usable enough to generate realistic incidents.
4. Prefer changes that improve observability, traceability, reproducibility, and safety.
5. Avoid broad feature expansion that does not clearly support the thesis goals.

If a task is ambiguous, prefer the option that strengthens the AIOps, RAG, or DevSecOps story.

If a fast shortcut would make the service harder to deploy, test, debug, or extend, do not take it unless the user explicitly asks for a temporary spike.

## 3. Required Reading Order

Before making substantial changes, agents should read the relevant docs instead of guessing.

Suggested reading order:

1. `PROJECT_CONTEXT.md`
2. `PROJECT_PROGRESS.md`
3. `README.md`
4. `docs/architecture/product-design.md`
5. `docs/architecture/service-boundaries.md`
6. `docs/architecture/data-ownership.md`
7. `docs/architecture/database-design.md`
8. `docs/architecture/repo-structure.md`
9. `docs/api/rest-api-design.md`
10. `docs/development/product-code-rules.md`
11. `docs/development/dependency-versioning.md`
12. Service-specific README and plan docs when working inside a service

At minimum, an agent must read the docs that directly affect the files being changed.

For product feature work, `docs/development/product-code-rules.md` is mandatory reading. It defines the default standard for code organization, API shape, persistence, state machines, observability, configuration, testing, and readiness for real deployment.

`PROJECT_PROGRESS.md` is the living handoff log. Agents must read it before coding to understand what was already done, what decisions were made, and what should happen next. After substantial work, agents should update it with a concise dated entry.

## 4. Scope Discipline

This repo is intentionally broad, so agents must actively control scope.

Rules:

- Prefer one complete end-to-end flow over many partial features.
- Do not add product features only because they seem impressive.
- Treat the microservices app as a realistic operational environment, not as an infinite product surface.
- Do not expand livestream, social, mobile, or admin features unless they support the core thesis flow or the current requested task.
- If a simpler implementation supports the same thesis objective, choose the simpler implementation.

The app should be realistic, not inflated. A feature is valuable when it strengthens a real product flow, creates useful operational evidence, or supports the thesis evaluation.

## 5. Production-Grade Code Organization

Every code change should leave the repository closer to a system that can run in a real environment.

For Go product services, prefer this layout:

- `cmd/server`: process entrypoint, wiring, graceful shutdown.
- `internal/config`: environment loading, defaults, validation.
- `internal/domain`: domain models, state constants, invariants, domain errors.
- `internal/handler`: HTTP request/response transport, validation boundary, status mapping.
- `internal/service`: use cases and business workflow orchestration.
- `internal/repository`: persistence interfaces and database implementations.
- `internal/event`: event names, payloads, producers, consumers, outbox helpers where needed.
- `internal/observability`: middleware, logging, metrics, readiness helpers.
- `migrations`: schema changes owned by the service.
- `tests`: integration, smoke, or service-level test assets when package-local tests are not enough.

For Python AIOps code, keep separation between:

- `app/api`: HTTP API surface.
- `app/core`: configuration and shared runtime concerns.
- `app/collectors`: external evidence collectors.
- `app/agents`: agent orchestration and specialist agent logic.
- `app/rag`: chunking, embedding, retrieval, vector-store concerns.
- `app/redaction`: secret and sensitive-data filtering.
- `app/scoring`: confidence, ranking, and evaluation helpers.
- `app/schemas`: request, response, evidence, and RCA schemas.

Layering rules:

- `cmd/server` may wire dependencies, but should not contain product logic.
- Handlers should not contain business workflow logic.
- Services should not know HTTP details unless unavoidable.
- Repositories should not enforce product policy beyond persistence constraints.
- Domain types should make invalid states harder to express.
- Cross-service behavior must use APIs, events, or contracts, not shared databases.

Do not collapse new production behavior into a single file only because the current service is small. Small services can start simple, but their code should still point in the direction of this layout.

## 6. Architecture Rules

- Respect bounded contexts described in `docs/architecture/service-boundaries.md`.
- Do not move business logic into `api-gateway`.
- Do not make services read each other's databases directly.
- Cross-service coordination should happen through APIs, events, or explicit contracts.
- Every meaningful workflow should preserve request, correlation, and entity identifiers.
- Video lifecycle, job lifecycle, and incident lifecycle must use explicit states rather than inferred states.
- Remediation logic must remain advisory unless the user explicitly asks for automation.

## 7. Data and Storage Rules

- Follow `docs/architecture/database-design.md`.
- PostgreSQL is the source of truth for transactional and lifecycle state.
- MongoDB is for flexible documents such as incidents, evidence, RCA reports, comments, or read models where applicable.
- Redis is not a durable source of truth.
- Never introduce cross-service foreign keys at the database layer.
- Prefer storing references by stable IDs such as `user_id`, `video_id`, `job_id`, or `incident_id`.
- Timestamps should be stored in UTC.
- Never store secrets, tokens, presigned URLs, stream keys, or sensitive credentials in logs or persisted evidence.

## 8. API and Contract Rules

- Follow `docs/api/rest-api-design.md`.
- Public API paths go through `api-gateway` under `/api/v1/*`.
- Internal service APIs should remain under `/v1/*`.
- Use plural resource naming and snake_case JSON fields.
- For new endpoints, define stable error codes and preserve request IDs in responses where the local convention expects them.
- If an API change affects docs or contracts, update the relevant documentation in the same change when practical.

## 9. Runtime and Deployment Rules

Production-shaped code must be able to run consistently across local, dev, demo, and Kubernetes.

- Load runtime behavior from explicit configuration.
- Validate required configuration at startup.
- Fail fast in non-local environments when critical dependencies or secrets are missing.
- Provide `/healthz`, `/readyz`, and `/metrics` for services that run as HTTP processes.
- Use timeouts for network calls, database calls, and upstream requests.
- Use structured logs with service name, environment, request ID, correlation ID, and entity IDs when available.
- Keep local fallbacks clearly labeled as local-only.
- Do not hard-code local URLs, secrets, bucket names, image names, or environment assumptions in product logic.

If a feature depends on PostgreSQL, Redis, object storage, a queue, or another service, treat that dependency as unreliable and code the failure path deliberately.

## 10. Service-Specific Guidance

### `services/api-gateway`

- Keep it focused on routing, request context, policy enforcement, and observability.
- Do not place product business logic here.
- Prefer middleware and transport concerns only.

### `services/identity-service`

- PostgreSQL is the canonical source of truth outside local-only fallback flows.
- Redis rate limiting should fail safely and predictably.
- Changes here should preserve security posture, especially around JWT, refresh flows, OAuth, and auditability.

### `services/video-service`

- Owns upload intent, metadata, object keys, and lifecycle state.
- Must not perform FFmpeg processing directly.

### `services/media-worker`

- Owns processing jobs, attempts, retries, dead-letter behavior, and worker state.
- Should interact with canonical video state through controlled APIs or events.

### `services/feed-social-service`

- Keep feed and basic social behavior simple unless there is a thesis-driven reason to expand it.

### `services/live-service`

- Build only what is needed for realistic demo and incident generation.

### `services/aiops-service`

- This is the thesis core.
- Prefer clean separation between collectors, agents, schemas, scoring, redaction, and RAG pipeline code.
- Every agent conclusion should be traceable to evidence.
- Avoid claims without evidence references.
- Prefer designs that are easy to evaluate against baselines.

## 11. Multi-Agent RAG Rules

When implementing AIOps or RCA logic:

- Keep agent responsibilities explicit and non-overlapping where possible.
- Planner-like logic should coordinate the investigation.
- Specialist agents should focus on distinct evidence types such as logs, metrics, deployments, Kubernetes state, or runbooks.
- Final RCA synthesis should cite supporting evidence.
- Include confidence or uncertainty where appropriate.
- Prefer grounded outputs over fluent but weakly supported outputs.
- Reduce hallucination risk through retrieval, validation, and cross-checking.

When adding or changing agents, clearly define:

- Inputs
- Evidence sources
- Expected outputs
- Failure modes
- Evaluation implications

## 12. DevSecOps and GitOps Rules

- Treat CI, image metadata, scan output, manifest diffs, and deployment history as first-class evidence sources.
- The source repo builds, tests, scans, and packages artifacts.
- The GitOps repo controls desired deployment state and Argo CD sync behavior.
- Do not bypass GitOps assumptions when making deployment-related design decisions unless the user explicitly asks for a local-only shortcut.
- Security tooling should support the thesis story, not exist only as decoration.

## 13. Testing and Verification Rules

Agents should scale testing to the risk of the change.

- Small local refactors: run focused tests for the touched module.
- Behavior changes: run relevant unit or integration tests where available.
- Contract or workflow changes: verify the affected path end to end as much as practical.
- If tests cannot be run, say so clearly.

When adding meaningful logic, prefer adding or updating tests close to the touched code.

## 14. Documentation Rules

This project depends heavily on docs for both engineering alignment and thesis writing.

Agents should update docs when changes affect:

- architecture decisions
- service responsibilities
- API shape
- event contracts
- dependency versions
- experiment or incident reproduction flow
- project progress, handoff state, or next-step priorities

Do not create documentation churn for tiny internal refactors, but do keep docs aligned with real behavior.

## 15. Dependency Rules

- Follow `docs/development/dependency-versioning.md`.
- Do not introduce floating versions in production manifests.
- Prefer the standard library or existing project patterns before adding new libraries.
- If a new direct dependency is added, update the relevant manifest and versioning docs when required by current repo policy.

## 16. Editing Rules for Agents

- Keep changes narrow and intentional.
- Do not perform unrelated refactors while working on a scoped task.
- Preserve the existing structure unless there is a strong reason to change it.
- Prefer consistency with the surrounding code over personal style.
- Leave short comments only when they genuinely improve readability.

## 17. Completion Checklist

Before finishing a task, agents should check:

- Is the code organized for real service growth instead of a quick demo?
- Can this run outside the author's machine with explicit configuration?
- Did I respect service boundaries and storage rules?
- Did I keep business logic out of handlers and gateways where practical?
- Did I include clear state transitions, error paths, and dependency failure behavior where relevant?
- Did I preserve observability through logs, metrics, readiness, request IDs, and correlation IDs?
- Does this change support the thesis direction or the requested feature directly?
- Did I avoid pushing product scope wider than necessary?
- Did I update tests if behavior changed?
- Did I update docs if contracts, architecture, or dependencies changed?
- Did I update `PROJECT_PROGRESS.md` if this was substantial work or changed next-step priorities?
- Can the user explain or demo this change easily in the context of the thesis?

## 18. Preferred Default Interpretation

If the user says something like:

- "read the rules and code"
- "follow the repo rules"
- "code according to project rules"
- "code product-style"
- "make it production-grade"

Agents should treat this file as the default authority for implementation style and decision-making in this repository.

If the user asks for product code, agents should also treat `docs/development/product-code-rules.md` as required authority.
