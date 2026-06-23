# Repo Structure

Repo này là monorepo cho source code, product apps, AIOps service, tooling và docs. Repo GitOps riêng quản lý trạng thái deploy thật.

## Top-Level Layout

```text
services/      backend services, workers, aiops service
apps/          admin web and mobile app
packages/      contracts and shared conventions
deploy/        local compose, k8s templates, helm templates, scripts
infra/         platform notes and local lab setup
docs/          architecture, runbooks, incidents, experiments, thesis
tests/         e2e, smoke and load tests
tools/         incident injector, log generator, RCA evaluator
```

## Standard Product Service Layout

Product services should be organized for real deployment and long-term growth, even when the current implementation is still small.

Preferred Go service layout:

```text
services/<service-name>/
├── cmd/server/             process entrypoint and dependency wiring
├── internal/config/        environment loading and validation
├── internal/domain/        domain models, states, invariants, domain errors
├── internal/handler/       HTTP transport and request/response mapping
├── internal/service/       use cases and business workflows
├── internal/repository/    persistence interfaces and implementations
├── internal/event/         event contracts, producers, consumers, outbox helpers
├── internal/observability/ logging, metrics, middleware, readiness helpers
├── migrations/             service-owned schema migrations
├── tests/                  integration, smoke, or service-level test assets
├── Dockerfile
├── go.mod
└── README.md
```

Layering rules:

- `cmd/server` wires dependencies and starts the process; it should not contain business logic.
- `handler` maps HTTP to use cases; it should not own workflow decisions.
- `service` owns business workflow and coordinates repositories/events.
- `repository` owns persistence access; it should not contain product policy beyond data constraints.
- `domain` owns states, invariants and stable domain errors.
- `event` owns asynchronous contracts; services should not depend on another service database for state propagation.

The current service scaffolds intentionally include these directories so new code has a consistent place to land instead of accumulating in `main.go`.

## AIOps Service Layout

`services/aiops-service` follows a Python layout because it owns RCA, collectors, RAG, and agent orchestration.

```text
services/aiops-service/
├── app/api/          HTTP API surface
├── app/core/         configuration and runtime concerns
├── app/collectors/   Kubernetes, Loki, Prometheus, Argo CD, GitLab, scan collectors
├── app/agents/       planner and specialist agent logic
├── app/rag/          chunking, embedding, retrieval, vector-store integration
├── app/redaction/    secret and sensitive-data filtering
├── app/scoring/      confidence, ranking, evaluation support
├── app/schemas/      request, response, evidence and RCA schemas
└── tests/
```

This service is the thesis core, so its structure should preserve clear boundaries between evidence collection, retrieval, agent reasoning, validation, scoring and report synthesis.

## Runtime Và Version

Runtime, image nền và dependency policy được cố định tại [Dependency Versioning](../development/dependency-versioning.md).

Baseline hiện tại:

- Go services: Go `1.24`, toolchain `go1.24.13`.
- `aiops-service`: Python `3.12.13`.
- Admin web và mobile app chưa scaffold chính thức; version sẽ được chốt trong tài liệu versioning trước khi thêm dependency.

## Quy Tắc Mở Rộng

- Service mới đặt trong `services/<name>`.
- Client app mới đặt trong `apps/<name>`.
- Contract chung đặt trong `packages/contracts`.
- Không tạo shared library cho business logic nếu chưa có nhu cầu thật.
- Manifest deploy thật sau cùng phải đi qua repo GitOps.
- Tài liệu incident và experiment phải đi cùng implementation để phục vụ đánh giá khóa luận.

## Production-Grade Organization Rules

- Code mới không nên gom vào một file lớn chỉ để chạy nhanh.
- Service phải giữ được boundary rõ ràng giữa handler, service, repository, domain, event và observability.
- Tính năng cần chạy trên Kubernetes phải có config rõ ràng, health/readiness, logging và failure path có thể debug.
- Migration, API contract, event contract và docs phải đi cùng thay đổi khi chúng ảnh hưởng hành vi thật.
- Local fallback được phép cho phát triển, nhưng phải được ghi rõ là local-only và không trở thành hành vi production ngầm.
