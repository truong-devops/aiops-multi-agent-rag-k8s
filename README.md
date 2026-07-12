# AIOps Multi-Agent RAG on Kubernetes

A Kubernetes-first video and livestream platform used as a practical environment for AIOps root cause analysis.

The project combines a microservices product with an Evidence-aware Multi-Agent RAG system. The product generates realistic operational signals such as logs, metrics, deployment changes, queue delays, storage errors, and worker failures. The AIOps layer uses those signals to investigate incidents and produce RCA reports with supporting evidence.

The main focus areas are:

- Product engineering for an API-gateway-fronted video/livestream platform.
- Kubernetes-based deployment and observability.
- DevSecOps and GitOps delivery workflows.
- Multi-Agent RAG for incident investigation and root cause analysis.

This repository contains the application source, service code, local development setup, documentation, testing assets, and supporting tools. Kubernetes desired state is managed separately in the companion GitOps repository.

Runtime and dependency versions are fixed in [Dependency Versioning](./docs/development/dependency-versioning.md). Current backend baseline: Go services use Go `1.24` with toolchain `go1.24.13`; `aiops-service` uses Python `3.12.13`.

For fast onboarding or context recovery, read [Project Context](./PROJECT_CONTEXT.md) and [Project Progress](./PROJECT_PROGRESS.md) before diving into the detailed architecture docs. Coding agents should read [AGENTS.md](./AGENTS.md) first.

## Repository Map

- `services/`: product backend services and the Python AIOps service.
- `apps/admin-web`: Next.js operations dashboard for video, livestream and AIOps RCA workflows.
- `apps/mobile-flutter`: Flutter end-user app shell for feed, upload and live viewing.
- `deploy/docker-compose`: local product stack for development and smoke testing.
- `docs/`: architecture, development rules, API notes, runbooks, incidents and thesis material.
- `packages/`: shared contracts, proto placeholders and shared documentation.
- `tests/`: cross-service smoke, e2e and load-test assets.
- `tools/`: incident injection, log generation and RCA evaluation utilities.

## Current Product Surface

- Public traffic goes through `api-gateway` under `/api/v1/*`.
- `identity-service` handles auth, users, sessions and JWT/JWKS.
- `video-service` owns upload intents, metadata, lifecycle state and video outbox events.
- `media-worker` consumes upload events, runs placeholder or FFmpeg processing, and reports status back to `video-service`.
- `feed-social-service` owns the ready-video feed, likes, comments, follows and optional Redis read cache.
- `live-service` owns live session create/list/start/end flows.
- `aiops-service` is the future RCA orchestration surface; product admin UI already reserves incident/RCA workflows for it.

## Verification

Common checks:

```bash
make test-video
make test-media
make test-live
make smoke-product
cd apps/admin-web && npm run lint && npm run typecheck && npm run build
cd apps/mobile-flutter && flutter analyze && flutter test
```

Some checks require Docker, Node `20.19.0+`, Flutter, or an installed Xcode iOS Simulator runtime.
