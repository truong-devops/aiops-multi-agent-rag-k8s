# AGENTS.md

This file defines the working rules for coding agents contributing to this repository.

When a user asks an agent to code in this repo, the agent should read this file first and follow it unless the user explicitly asks to do otherwise.

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

1. Protect the thesis focus: incident analysis, evidence collection, RCA quality, evaluation readiness.
2. Keep the core product flow usable enough to generate realistic incidents.
3. Prefer changes that improve observability, traceability, reproducibility, and safety.
4. Avoid broad feature expansion that does not clearly support the thesis goals.

If a task is ambiguous, prefer the option that strengthens the AIOps, RAG, or DevSecOps story.

## 3. Required Reading Order

Before making substantial changes, agents should read the relevant docs instead of guessing.

Suggested reading order:

1. `README.md`
2. `docs/architecture/product-design.md`
3. `docs/architecture/service-boundaries.md`
4. `docs/architecture/database-design.md`
5. `docs/api/rest-api-design.md`
6. `docs/development/dependency-versioning.md`
7. Service-specific README and plan docs when working inside a service

At minimum, an agent must read the docs that directly affect the files being changed.

## 4. Scope Discipline

This repo is intentionally broad, so agents must actively control scope.

Rules:

- Prefer one complete end-to-end flow over many partial features.
- Do not add product features only because they seem impressive.
- Treat the microservices app as a realistic operational environment, not as an infinite product surface.
- Do not expand livestream, social, mobile, or admin features unless they support the core thesis flow or the current requested task.
- If a simpler implementation supports the same thesis objective, choose the simpler implementation.

## 5. Architecture Rules

- Respect bounded contexts described in `docs/architecture/service-boundaries.md`.
- Do not move business logic into `api-gateway`.
- Do not make services read each other's databases directly.
- Cross-service coordination should happen through APIs, events, or explicit contracts.
- Every meaningful workflow should preserve request, correlation, and entity identifiers.
- Video lifecycle, job lifecycle, and incident lifecycle must use explicit states rather than inferred states.
- Remediation logic must remain advisory unless the user explicitly asks for automation.

## 6. Data and Storage Rules

- Follow `docs/architecture/database-design.md`.
- PostgreSQL is the source of truth for transactional and lifecycle state.
- MongoDB is for flexible documents such as incidents, evidence, RCA reports, comments, or read models where applicable.
- Redis is not a durable source of truth.
- Never introduce cross-service foreign keys at the database layer.
- Prefer storing references by stable IDs such as `user_id`, `video_id`, `job_id`, or `incident_id`.
- Timestamps should be stored in UTC.
- Never store secrets, tokens, presigned URLs, stream keys, or sensitive credentials in logs or persisted evidence.

## 7. API and Contract Rules

- Follow `docs/api/rest-api-design.md`.
- Public API paths go through `api-gateway` under `/api/v1/*`.
- Internal service APIs should remain under `/v1/*`.
- Use plural resource naming and snake_case JSON fields.
- For new endpoints, define stable error codes and preserve request IDs in responses where the local convention expects them.
- If an API change affects docs or contracts, update the relevant documentation in the same change when practical.

## 8. Service-Specific Guidance

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

## 9. Multi-Agent RAG Rules

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

## 10. DevSecOps and GitOps Rules

- Treat CI, image metadata, scan output, manifest diffs, and deployment history as first-class evidence sources.
- The source repo builds, tests, scans, and packages artifacts.
- The GitOps repo controls desired deployment state and Argo CD sync behavior.
- Do not bypass GitOps assumptions when making deployment-related design decisions unless the user explicitly asks for a local-only shortcut.
- Security tooling should support the thesis story, not exist only as decoration.

## 11. Testing and Verification Rules

Agents should scale testing to the risk of the change.

- Small local refactors: run focused tests for the touched module.
- Behavior changes: run relevant unit or integration tests where available.
- Contract or workflow changes: verify the affected path end to end as much as practical.
- If tests cannot be run, say so clearly.

When adding meaningful logic, prefer adding or updating tests close to the touched code.

## 12. Documentation Rules

This project depends heavily on docs for both engineering alignment and thesis writing.

Agents should update docs when changes affect:

- architecture decisions
- service responsibilities
- API shape
- event contracts
- dependency versions
- experiment or incident reproduction flow

Do not create documentation churn for tiny internal refactors, but do keep docs aligned with real behavior.

## 13. Dependency Rules

- Follow `docs/development/dependency-versioning.md`.
- Do not introduce floating versions in production manifests.
- Prefer the standard library or existing project patterns before adding new libraries.
- If a new direct dependency is added, update the relevant manifest and versioning docs when required by current repo policy.

## 14. Editing Rules for Agents

- Keep changes narrow and intentional.
- Do not perform unrelated refactors while working on a scoped task.
- Preserve the existing structure unless there is a strong reason to change it.
- Prefer consistency with the surrounding code over personal style.
- Leave short comments only when they genuinely improve readability.

## 15. Completion Checklist

Before finishing a task, agents should check:

- Does this change support the thesis direction or the requested feature directly?
- Did I respect service boundaries and storage rules?
- Did I avoid pushing product scope wider than necessary?
- Did I update tests if behavior changed?
- Did I update docs if contracts, architecture, or dependencies changed?
- Can the user explain or demo this change easily in the context of the thesis?

## 16. Preferred Default Interpretation

If the user says something like:

- "read the rules and code"
- "follow the repo rules"
- "code according to project rules"

Agents should treat this file as the default authority for implementation style and decision-making in this repository.
