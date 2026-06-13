# Services

Thư mục này chứa backend services và worker của sản phẩm video/livestream.

Backend product services dự kiến dùng Go. `aiops-service` dự kiến dùng Python FastAPI vì thuận lợi hơn cho RAG, agent orchestration, embedding và data processing.

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
