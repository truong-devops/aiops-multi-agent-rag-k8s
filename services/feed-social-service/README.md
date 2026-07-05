# feed-social-service

Service phuc vu ready-video feed va cac tuong tac xa hoi co ban.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Purpose

`feed-social-service` dong vai tro read/social layer cho product flow:

- Hien thi video da xu ly xong va public trong feed.
- Luu feed read model rieng, khong doc database cua `video-service`.
- Lam nen tang cho likes, comments, follows, counters va cache sau nay.
- Tao operational evidence cho AIOps nhu slow feed query, stale read model, duplicate event va dependency outage.

## Current Implementation

Da co:

- Config loading va validation trong `internal/config`.
- Structured JSON logging, request/correlation middleware, body limit va graceful shutdown.
- `/healthz`, `/readyz`, `/metrics`.
- Local in-memory store cho local/test.
- PostgreSQL store khi `DATABASE_URL` duoc cau hinh.
- Migration `migrations/001_feed_schema.sql` cho `feed_items`, `video_social_counters`, `inbox_events`.
- Idempotent ready-video feed item upsert theo `event_id`.
- Feed list repository query theo `ready_at DESC, video_id DESC`.
- Unit tests va skipped-by-default PostgreSQL integration harness.

Chua co:

- `GET /v1/feed`.
- Kafka/Redpanda consumer cho `video.ready.v1`.
- Controlled internal ingestion API.
- Likes, comments, follows.
- Redis cache va MongoDB comments/read model.

## API

Current direct service routes:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Planned direct service routes:

- `GET /v1/feed?limit=&cursor=`
- `GET /v1/videos/{video_id}/social`
- `PUT /v1/videos/{video_id}/like`
- `DELETE /v1/videos/{video_id}/like`
- `GET /v1/videos/{video_id}/comments?limit=&cursor=`
- `POST /v1/videos/{video_id}/comments`
- `DELETE /v1/comments/{comment_id}`
- `PUT /v1/users/{user_id}/follow`
- `DELETE /v1/users/{user_id}/follow`

Public clients should call through `api-gateway` under `/api/v1/*`.

## Configuration

| Env var | Default | Required | Notes |
|---|---:|---|---|
| `PORT` | `8080` | yes | HTTP listen port. |
| `ENVIRONMENT` | `local` | yes | Non-local environments require `DATABASE_URL`. |
| `LOG_LEVEL` | `info` | no | Supports `debug`, `info`, `warn`, `error`. |
| `DATABASE_URL` | empty | non-local | Enables PostgreSQL store when set. Local empty value uses in-memory store. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | no | Request body cap for future write APIs. |
| `FEED_DEFAULT_LIMIT` | `20` | no | Baseline for future feed API. |
| `FEED_MAX_LIMIT` | `50` | no | Baseline cap for future feed API. |

Planned env vars for later phases:

- `KAFKA_BROKERS`
- `VIDEO_EVENTS_TOPIC`
- `SOCIAL_EVENTS_TOPIC`
- `CONSUMER_GROUP`
- `CONSUMER_ENABLED`
- `INTERNAL_API_TOKEN`
- `REDIS_URL`
- `MONGODB_URI`
- `MONGODB_DATABASE`

## Development

Run tests:

```bash
go test ./...
```

Run PostgreSQL integration tests when a test database is available:

```bash
FEED_SOCIAL_TEST_DATABASE_URL='postgres://user:password@localhost:5432/feed_social_test?sslmode=disable' go test ./internal/repository -run TestPostgresStore -count=1
```

Run locally with in-memory store:

```bash
PORT=8080 ENVIRONMENT=local go run ./cmd/server
```

Run locally with PostgreSQL:

```bash
DATABASE_URL='postgres://user:password@localhost:5432/feed_social?sslmode=disable' go run ./cmd/server
```

## Incident Scenarios

This service should eventually generate useful evidence for:

- Slow feed query or missing index.
- PostgreSQL unavailable.
- Stale feed because `video.ready.v1` consumer is stopped.
- Duplicate ready events.
- Redis unavailable or cache stampede after cache phase.
- Social counter mismatch after failed write path.
