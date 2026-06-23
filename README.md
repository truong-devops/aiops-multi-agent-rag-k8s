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
