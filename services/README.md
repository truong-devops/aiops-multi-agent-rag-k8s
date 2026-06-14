# Services

Thư mục này chứa backend services và worker của sản phẩm video/livestream.

Backend product services dùng Go. `aiops-service` dùng Python FastAPI vì thuận lợi hơn cho RAG, agent orchestration, embedding và data processing.

Runtime baseline hiện tại:

- Go services: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.
- `aiops-service`: Python `3.12.13`, Docker runtime `python:3.12.13-slim-bookworm`.

Chi tiết version và dependency policy được cố định ở [Dependency Versioning](../docs/development/dependency-versioning.md). Khi đổi runtime hoặc dependency trực tiếp, cập nhật tài liệu đó trong cùng thay đổi.

## Services

- `identity-service`: auth, user profile, JWT/session.
- `api-gateway`: public API entrypoint, routing, request context, auth verification later.
- `video-service`: upload request, video metadata, MinIO integration, event publishing.
- `media-worker`: consume video events, FFmpeg processing, thumbnail, retry/dead-letter.
- `feed-social-service`: feed, like/comment/follow ở mức product demo.
- `live-service`: live session, stream key, MediaMTX integration.
- `aiops-service`: collectors, RAG, agents, RCA scoring, redaction.

Service boundaries and ownership are defined in:

- [Product Design](../docs/architecture/product-design.md)
- [Service Boundaries](../docs/architecture/service-boundaries.md)
- [Data Ownership](../docs/architecture/data-ownership.md)
- [Event Contracts](../packages/contracts/event-contracts.md)
- [API Gateway Plan](../docs/service/api-gateway-plan.md)
- [Dependency Versioning](../docs/development/dependency-versioning.md)

## Convention Cho Go Service

```text
<service>/
├── cmd/server/
├── internal/
│   ├── config/
│   ├── domain/
│   ├── handler/
│   ├── repository/
│   ├── service/
│   ├── event/
│   └── observability/
├── migrations/
├── tests/
├── Dockerfile
├── go.mod
└── README.md
```

## Contract Bắt Buộc

Mỗi service khi bắt đầu code cần có:

- `/healthz`
- `/readyz`
- `/metrics`
- structured JSON logs
- `request_id` hoặc `trace_id`
- config qua environment variables
- Dockerfile multi-stage
- README mô tả API, dependencies và incident có thể sinh
