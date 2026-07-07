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
- `GET /v1/feed` voi limit cap, cursor pagination va response envelope.
- Kafka/Redpanda consumer cho `video.ready.v1`, chi chay khi `CONSUMER_ENABLED=true`.
- Controlled internal ingestion API `POST /v1/internal/feed-items` duoc bao ve bang `X-Internal-Token`.
- Likes idempotent voi durable `like_count`.
- PostgreSQL comments MVP voi create/list/delete va durable `comment_count`.
- Metrics cho feed operations, item count va event age.
- Unit tests va skipped-by-default PostgreSQL integration harness.

Chua co:

- Follows.
- Redis cache va MongoDB comments/read model.

## API

Current direct service routes:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `GET /v1/feed?limit=&cursor=`
- `GET /v1/videos/{video_id}/social`
- `PUT /v1/videos/{video_id}/like`
- `DELETE /v1/videos/{video_id}/like`
- `GET /v1/videos/{video_id}/comments?limit=&cursor=`
- `POST /v1/videos/{video_id}/comments`
- `DELETE /v1/comments/{comment_id}`
- `POST /v1/internal/feed-items`

Planned direct service routes:

- `PUT /v1/users/{user_id}/follow`
- `DELETE /v1/users/{user_id}/follow`

Public clients should call through `api-gateway` under `/api/v1/*`.
Write routes for likes/comments require trusted `X-User-ID` from `api-gateway`.

## Configuration

| Env var | Default | Required | Notes |
|---|---:|---|---|
| `PORT` | `8080` | yes | HTTP listen port. |
| `ENVIRONMENT` | `local` | yes | Non-local environments require `DATABASE_URL`. |
| `LOG_LEVEL` | `info` | no | Supports `debug`, `info`, `warn`, `error`. |
| `DATABASE_URL` | empty | non-local | Enables PostgreSQL store when set. Local empty value uses in-memory store. |
| `KAFKA_BROKERS` | empty | when consumer enabled | Comma-separated Redpanda/Kafka brokers. |
| `VIDEO_EVENTS_TOPIC` | `video-events` | when consumer enabled | Topic that carries `video.ready.v1`. |
| `CONSUMER_GROUP` | `feed-social-service` | when consumer enabled | Kafka consumer group. |
| `CONSUMER_ENABLED` | `false` | no | Enables ready-video event consumer. |
| `INTERNAL_API_TOKEN` | empty | for internal ingestion | Required by `POST /v1/internal/feed-items`. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | no | Request body cap for future write APIs. |
| `FEED_DEFAULT_LIMIT` | `20` | no | Default page size for feed API. |
| `FEED_MAX_LIMIT` | `50` | no | Maximum page size for feed API. |

Planned env vars for later phases:

- `SOCIAL_EVENTS_TOPIC`
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

Run with ready-video consumer:

```bash
DATABASE_URL='postgres://user:password@localhost:5432/feed_social?sslmode=disable' \
KAFKA_BROKERS='localhost:9092' \
VIDEO_EVENTS_TOPIC='video-events' \
CONSUMER_GROUP='feed-social-service' \
CONSUMER_ENABLED=true \
go run ./cmd/server
```

Seed a ready feed item through the controlled local/dev ingestion API after starting the service with `INTERNAL_API_TOKEN=local-secret`:

```bash
curl -X POST http://localhost:8080/v1/internal/feed-items \
  -H 'Content-Type: application/json' \
  -H 'X-Internal-Token: local-secret' \
  -d '{
    "event_id": "evt_local_1",
    "video_id": "vid_123",
    "owner_id": "usr_123",
    "title": "Ready video",
    "thumbnail_object_key": "thumbnails/vid_123/poster.jpg",
    "playback_object_key": "processed/vid_123/source.mp4",
    "duration_ms": 12340,
    "ready_at": "2026-07-06T10:00:00Z"
  }'
```

`POST /v1/internal/feed-items` is a controlled local/dev and MVP fallback. Prefer `video.ready.v1` consumption for real event-driven flow, and remove or restrict the fallback once the upload-to-processing-to-feed path is fully covered by events in deployment.

Like and unlike a ready video:

```bash
curl -X PUT http://localhost:8080/v1/videos/vid_123/like -H 'X-User-ID: usr_123'
curl -X DELETE http://localhost:8080/v1/videos/vid_123/like -H 'X-User-ID: usr_123'
```

Create and list comments:

```bash
curl -X POST http://localhost:8080/v1/videos/vid_123/comments \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: usr_123' \
  -d '{"body":"hello feed"}'

curl http://localhost:8080/v1/videos/vid_123/comments
```

## Incident Scenarios

This service should eventually generate useful evidence for:

- Slow feed query or missing index.
- PostgreSQL unavailable.
- Stale feed because `video.ready.v1` consumer is stopped.
- Duplicate ready events.
- Redis unavailable or cache stampede after cache phase.
- Social counter mismatch after failed write path.
