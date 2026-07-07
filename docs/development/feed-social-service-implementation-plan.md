# feed-social-service Implementation Plan

Tai lieu nay chi theo doi rieng `feed-social-service`. Dung file nay khi can tiep tuc phat trien feed va social layer ma khong muon bi lan sang plan tong the cua du an.

Legend:

- `[x]` Done
- `[~]` In progress / partial
- `[ ]` Not started

## Service Purpose

`feed-social-service` phuc vu feed video da san sang va cac tuong tac xa hoi co ban.

Service nay phuc vu hai muc tieu:

- Hoan thien product flow toi thieu: `upload -> process -> ready -> visible in feed`.
- Tao operational evidence cho AIOps/RCA: feed latency, stale read model, cache miss/stampede, Redis/Mongo/PostgreSQL outage, event lag, duplicate like/comment/follow attempts.

`feed-social-service` so huu feed read model, likes, comments, follows va social counters. Service nay khong so huu canonical video lifecycle, khong doc database cua `video-service`, va khong xu ly video file.

## Current Snapshot

As of 2026-07-07:

- `[x]` Da co Go module skeleton.
- `[x]` Da co `cmd/server/main.go` production-shaped voi config loading, structured logging, graceful shutdown, body limit, request/correlation middleware va store wiring.
- `[x]` Da co folder layout theo huong production: `internal/config`, `internal/domain`, `internal/event`, `internal/handler`, `internal/observability`, `internal/repository`, `internal/service`, `migrations`, `tests`.
- `[x]` Da co config loader/validation cho runtime scaffold va feed limit baseline.
- `[~]` Da co domain model cho feed item, social counters, inbox idempotency, likes va comments; follow domain se lam o phase social sau.
- `[x]` Da co PostgreSQL implementation cho feed read model; MongoDB/Redis chua them vi chua can cho MVP feed durable.
- `[x]` Da co API `GET /v1/feed` voi limit cap, cursor pagination va response envelope.
- `[x]` Da co ingestion tu `video.ready.v1` qua Kafka/Redpanda consumer va fallback `POST /v1/internal/feed-items` co `X-Internal-Token`.
- `[~]` Da co likes/comments MVP voi durable counters; follows chua co.
- `[~]` Da co metrics/logs scaffold cho HTTP, DB, feed operations, item count va event age; cache/social metrics se them theo cac phase sau.

## Boundary

`feed-social-service` owns:

- Feed read model.
- Likes.
- Comments.
- Follows.
- Durable social counters.
- Feed/cache runtime evidence.

`feed-social-service` may call or consume:

- `video.ready.v1` or `video.status_changed.v1` when lifecycle event publishing is ready.
- `video-service` controlled API for a temporary MVP sync path, but not its database.
- Redis for feed cache, hot counters and short-lived dedup keys.
- MongoDB for comments and flexible feed read model if needed.
- PostgreSQL for likes, follows and durable counters.

`feed-social-service` must not:

- Read or write `video-service` database.
- Own upload/processing status.
- Process video files.
- Store presigned URLs or internal tokens.
- Become a recommendation engine too early.
- Hide feed state only in Redis.

## Target Runtime Model

Process roles:

- HTTP server exposes `/healthz`, `/readyz`, `/metrics`.
- Feed API lists ready/active feed items.
- Optional event consumer builds or updates feed read model from video lifecycle events.
- Social write APIs update durable PostgreSQL/MongoDB state first, then update counters/cache.

Recommended first deploy shape:

- One binary.
- HTTP server always on.
- Event consumer can be disabled for local testing.
- PostgreSQL first for MVP feed/items/counters to reduce dependency count.
- MongoDB comments/feed snapshots can be added after the ready-video feed works.
- Redis cache can be added after durable behavior is stable.

## Product Strategy

MVP should be intentionally small:

1. Expose a feed of ready public videos.
2. Keep read model inside `feed-social-service`.
3. Add like/comment/follow after feed visibility is stable.
4. Add cache and richer ranking after correctness is tested.

Do not build recommendation, search, moderation, notification or personalized ranking until the basic app flow runs reliably.

## Event Contracts

Incoming lifecycle events:

- `[x]` `video.ready.v1`
- `[ ]` `video.deleted.v1` or `video.visibility_changed.v1` later if video removal/visibility changes need feed updates.

Expected `video.ready.v1` envelope:

- `event_id`
- `event_name`
- `event_version`
- `event_type`
- `aggregate_id`
- `producer`
- `environment`
- `correlation_id`
- `request_id`
- `occurred_at`
- `payload`

Expected `video.ready.v1` payload for feed read model:

- `video_id`
- `owner_id`
- `processed_object_key`
- `thumbnail_object_key`
- `duration_ms`
- `width`
- `height`
- `size_bytes`
- `ready_at` if available, otherwise use `occurred_at`

Important current decision:

- `video-service` remains the canonical lifecycle event producer.
- `media-worker` currently updates video status through `video-service`.
- If `video.ready.v1` is not published yet, `feed-social-service` can initially expose controlled admin/dev ingestion or query a controlled `video-service` API, but it must not read the video database directly.

Outgoing events:

- `[ ]` `video.liked.v1`
- `[ ]` `video.unliked.v1`
- `[ ]` `comment.created.v1`
- `[ ]` `comment.deleted.v1`
- `[ ]` `user.followed.v1`
- `[ ]` `user.unfollowed.v1`

Do not include access tokens, internal tokens, presigned URLs, raw comment moderation secrets or private user data in events.

## State Machines

Feed item lifecycle:

```text
active -> hidden -> deleted
```

Like lifecycle:

```text
active -> deleted
deleted -> active
```

Follow lifecycle:

```text
active -> deleted
active -> blocked
deleted -> active
```

Comment lifecycle:

```text
visible -> hidden -> deleted
visible -> blocked
```

Rules:

- Feed items must only expose videos that are ready and public.
- Like and follow operations should be idempotent.
- Comments should preserve deleted/hidden evidence without leaking deleted body where not needed.
- Counters must be derived from durable writes, not only Redis.
- Feed can be eventually consistent, but staleness must be visible through metrics/logs.

## Data Ownership Target

PostgreSQL tables owned by `feed-social-service`:

- `[x]` `feed_items` for MVP read model if keeping dependencies small.
- `[x]` `likes`
- `[x]` `follows`
- `[x]` `comments` for PostgreSQL MVP comments.
- `[x]` `video_social_counters`
- `[x]` `inbox_events` for event idempotency.
- `[ ]` `outbox_events` if social events are published.

MongoDB collections owned by `feed-social-service`:

- `[ ]` `comments`
- `[ ]` `feed_items` if using flexible read model later.

Redis planned keys:

- `[ ]` `feed:home:{user_id}`
- `[x]` `feed:guest`
- `[x]` `feed:video:{video_id}:counters`
- `[ ]` `feed:dedup:like:{video_id}:{user_id}`
- `[ ]` `feed:event:{event_id}`

Storage choice for first implementation:

- Prefer PostgreSQL-only MVP for `feed_items`, `likes`, `comments`, `follows`, and `video_social_counters`.
- Add MongoDB comments later only if comments need flexible nesting/read models.
- Add Redis after durable behavior and metrics are stable. Current implementation has optional Redis cache for guest feed and social counters.

## API Surface

Direct service routes:

- `[x]` `GET /healthz`
- `[x]` `GET /readyz`
- `[x]` `GET /metrics`
- `[x]` `GET /v1/feed?limit=&cursor=`
- `[x]` `GET /v1/videos/{video_id}/social`
- `[x]` `PUT /v1/videos/{video_id}/like`
- `[x]` `DELETE /v1/videos/{video_id}/like`
- `[x]` `GET /v1/videos/{video_id}/comments?limit=&cursor=`
- `[x]` `POST /v1/videos/{video_id}/comments`
- `[x]` `DELETE /v1/comments/{comment_id}`
- `[x]` `PUT /v1/users/{user_id}/follow`
- `[x]` `DELETE /v1/users/{user_id}/follow`
- `[x]` Optional `POST /v1/internal/feed-items` for controlled MVP ingestion if `video.ready.v1` is not available yet.

Public clients should call through `api-gateway`:

- `GET /api/v1/feed`
- `PUT /api/v1/videos/{video_id}/like`
- `DELETE /api/v1/videos/{video_id}/like`
- `GET /api/v1/videos/{video_id}/comments`
- `POST /api/v1/videos/{video_id}/comments`
- `PUT /api/v1/users/{user_id}/follow`
- `DELETE /api/v1/users/{user_id}/follow`

Auth expectations:

- Public feed can allow anonymous reads.
- Likes, comments and follows require trusted user context from `api-gateway`.
- Internal ingestion routes require `X-Internal-Token`.

## Feed API Contract

`GET /v1/feed?limit=20&cursor=...`

Response shape:

```json
{
  "data": [
    {
      "video_id": "vid_...",
      "owner": {
        "id": "usr_...",
        "display_name": ""
      },
      "title": "Video title snapshot",
      "description": "Short description snapshot",
      "thumbnail_object_key": "thumbnails/vid_.../poster.jpg",
      "playback_object_key": "processed/vid_.../source.mp4",
      "duration_ms": 12340,
      "like_count": 0,
      "comment_count": 0,
      "ready_at": "2026-07-04T10:00:00Z"
    }
  ],
  "page": {
    "limit": 20,
    "next_cursor": "",
    "has_more": false
  },
  "request_id": "req_..."
}
```

Notes:

- Do not persist or return presigned URLs in MVP unless a controlled media URL strategy exists.
- Store object keys; URL generation can be added later at gateway/media layer.
- Cursor should be stable enough for pagination, such as encoded `ready_at` + `video_id`.

## Config Target

Required or planned env vars:

- `[x]` `PORT`
- `[x]` `ENVIRONMENT`
- `[x]` `LOG_LEVEL`
- `[x]` `DATABASE_URL`
- `[ ]` `MONGODB_URI`
- `[ ]` `MONGODB_DATABASE`
- `[x]` `REDIS_URL`
- `[x]` `KAFKA_BROKERS`
- `[x]` `VIDEO_EVENTS_TOPIC`
- `[ ]` `SOCIAL_EVENTS_TOPIC`
- `[x]` `CONSUMER_GROUP`
- `[x]` `CONSUMER_ENABLED`
- `[ ]` `VIDEO_SERVICE_BASE_URL`
- `[x]` `INTERNAL_API_TOKEN`
- `[x]` `REQUEST_BODY_LIMIT_BYTES`
- `[x]` `FEED_DEFAULT_LIMIT`
- `[x]` `FEED_MAX_LIMIT`
- `[x]` `CACHE_ENABLED`
- `[x]` `FEED_CACHE_TTL`

Validation rules:

- Non-local environments require PostgreSQL.
- Consumer cannot start without Kafka brokers/topic/group.
- Internal ingestion/API sync cannot start without `VIDEO_SERVICE_BASE_URL` and/or `INTERNAL_API_TOKEN`.
- MongoDB and Redis should be optional until the phase that uses them.
- Request body and feed limits must be positive and capped.

## Phase 1: Production-Shaped Service Scaffold

- `[x]` Add `internal/config` with env loading, defaults and validation.
- `[x]` Add `internal/domain` with feed item, counters, inbox idempotency models and errors; like/follow/comment models are deferred to their own phases.
- `[x]` Add `internal/observability` with request/correlation middleware and Prometheus text metrics.
- `[x]` Replace placeholder `cmd/server/main.go` with graceful shutdown wiring.
- `[x]` Add real `/healthz`, `/readyz`, `/metrics`.
- `[x]` Keep local mode explicit and safe.
- `[x]` Add config and domain unit tests.

Done criteria:

- Service starts locally with safe defaults.
- Non-local missing durable dependencies fails fast.
- Code layout matches repository production rules.

## Phase 2: Feed Read Model Persistence

- `[x]` Add migration `001_feed_schema.sql`.
- `[x]` Create `feed_items`.
- `[x]` Create `video_social_counters`.
- `[x]` Create `inbox_events` for event idempotency.
- `[x]` Implement repository interface.
- `[x]` Implement PostgreSQL store.
- `[x]` Implement in-memory store only for local/tests.
- `[x]` Add idempotent upsert for ready video feed item.
- `[x]` Add list feed query ordered by `ready_at DESC, video_id DESC`.
- `[x]` Add repository unit tests and skipped-by-default PostgreSQL integration harness.

Done criteria:

- Ready video can be represented in feed without reading video-service DB.
- Duplicate ready events do not duplicate feed items.
- Feed list is stable and paginated.

## Phase 3: Minimal Feed API

- `[x]` Add handler for `GET /v1/feed`.
- `[x]` Add cursor/limit parsing and validation.
- `[x]` Add service use case for list feed.
- `[x]` Add JSON response envelope with `data`, `page`, `request_id`.
- `[x]` Add route not found and method not allowed errors.
- `[x]` Add tests for empty feed, populated feed, limit cap and cursor behavior.

Done criteria:

- Public clients can list ready videos through `api-gateway`.
- Feed API returns only active ready feed items.
- Invalid query input fails with stable error code.

## Phase 4: Ready Video Ingestion

Preferred path:

- `[x]` Consume `video.ready.v1` from Redpanda/Kafka.
- `[x]` Parse and validate event envelope.
- `[x]` Validate required feed payload fields.
- `[x]` Insert `inbox_events` record by `event_id`.
- `[x]` Upsert feed item transactionally/idempotently.
- `[x]` Commit Kafka offset only after durable feed update.
- `[x]` Add metrics for consumed, duplicate, invalid, failed and event age.
- `[x]` Add tests with fake consumer/event handler.

Fallback path if lifecycle event is not available yet:

- `[x]` Add internal ingestion API guarded by `X-Internal-Token`.
- `[ ]` Or add controlled video-service API client for ready videos without direct DB access.
- `[x]` Document which fallback is active and when to remove it.

Done criteria:

- A processed ready video can appear in feed without cross-database coupling.
- Duplicate events or retries are safe.
- Feed staleness and ingestion failures are observable.

## Phase 5: Likes And Counters

- `[x]` Add `likes` table.
- `[x]` Add idempotent `PUT /v1/videos/{video_id}/like`.
- `[x]` Add idempotent `DELETE /v1/videos/{video_id}/like`.
- `[x]` Update `video_social_counters` transactionally from durable like writes.
- `[x]` Require trusted user context.
- `[x]` Add stable error codes for missing user, invalid video, invalid state and conflict.
- `[ ]` Add optional `video.liked.v1` / `video.unliked.v1` outbox contract.
- `[x]` Add handler/service/repository tests.

Done criteria:

- Repeated like/unlike calls are safe.
- Like count survives Redis loss or restart.
- Counter changes are visible in feed/social response.

## Phase 6: Comments

- `[x]` Decide PostgreSQL MVP comments vs MongoDB comments.
- `[x]` Add comment domain and validation.
- `[x]` Add `POST /v1/videos/{video_id}/comments`.
- `[x]` Add `GET /v1/videos/{video_id}/comments`.
- `[x]` Add `DELETE /v1/comments/{comment_id}`.
- `[x]` Update durable `comment_count`.
- `[x]` Add optional moderation-ready status fields.
- `[ ]` Add optional `comment.created.v1` / `comment.deleted.v1` outbox contract.
- `[x]` Add tests for create/list/delete and body validation.

Done criteria:

- Viewers can add and list comments for ready videos.
- Deleted/hidden comments do not leak body unexpectedly.
- Comment count is durable and observable.

## Phase 7: Follows

- `[x]` Add `follows` table.
- `[x]` Add idempotent `PUT /v1/users/{user_id}/follow`.
- `[x]` Add idempotent `DELETE /v1/users/{user_id}/follow`.
- `[x]` Reject following yourself.
- `[x]` Require trusted user context.
- `[ ]` Add optional `user.followed.v1` / `user.unfollowed.v1` outbox contract.
- `[x]` Add tests for follow/unfollow/self-follow.

Done criteria:

- Social graph facts are durable.
- Future personalized feed can build on follows.
- Follow behavior does not require identity DB reads.

## Phase 8: Cache And Performance

- `[x]` Add Redis client only after durable feed behavior works.
- `[x]` Add cache interface with no-op local implementation.
- `[x]` Cache public/guest feed with short TTL.
- `[x]` Cache hot social counters with short TTL.
- `[x]` Add cache invalidation on feed item/social write.
- `[x]` Add metrics for cache hit, miss, bypass, error and latency.
- `[x]` Add tests for cache fallback when Redis is unavailable.

Done criteria:

- Redis improves latency but is not required for correctness.
- Cache errors fail open for reads where safe.
- Cache stampede risk is visible through metrics.

## Phase 9: Observability And Incident Evidence

- `[~]` Add HTTP request counters/duration by method/path/status.
- `[~]` Add feed list latency and result count metrics.
- `[x]` Add ingestion event age and failure metrics.
- `[x]` Add DB operation latency/error metrics.
- `[x]` Add Redis cache metrics when cache exists.
- `[ ]` Add MongoDB operation metrics when comments/read model use MongoDB.
- `[~]` Add structured logs with `service`, `environment`, `request_id`, `correlation_id`, `user_id`, `video_id`, `comment_id`, `event_id`, `error_code`.
- `[ ]` Add optional OpenTelemetry trace propagation.

Done criteria:

- AIOps can diagnose stale feed, slow feed query, DB outage, Redis outage, duplicate event storm and social write failures.

## Phase 10: Deployment Readiness

- `[x]` Dockerfile exists.
- `[~]` Add service env documentation for DB, Kafka, Redis, MongoDB and internal token.
- `[ ]` Add local compose dependencies when needed.
- `[ ]` Add Kubernetes/GitOps manifests in companion repo when ready.
- `[ ]` Add liveness/readiness probes.
- `[ ]` Add secret/config references without hard-coded credentials.
- `[ ]` Add smoke test for ready video ingestion to feed listing.

Done criteria:

- Service can run in local compose and Kubernetes with the same config model.
- Missing required production dependency fails fast.
- Feed failures generate useful evidence instead of silent empty lists.

## Immediate Next Task

Next best engineering task:

1. Add local compose smoke test for ready video ingestion to feed listing plus like/comment/follow.
2. Decide whether optional social outbox events are needed.
3. Add Kubernetes/GitOps manifests and resource sizing for product services.
4. Move to `live-service` if the product flow is enough for the first demo.

Reason:

- `video-service` and `media-worker` can now produce ready videos.
- The product app still cannot show ready videos to users.
- A minimal feed closes the core product flow before touching AIOps/AI code.

## Update Rule

When working on `feed-social-service`:

- Read `AGENTS.md`, `PROJECT_CONTEXT.md`, `PROJECT_PROGRESS.md`, this file and `services/feed-social-service/README.md`.
- Update this checklist when a meaningful item changes.
- Update `PROJECT_PROGRESS.md` after substantial implementation work.
- Keep `docs/development/implementation-plan.md` as the high-level roadmap and this file as the detailed `feed-social-service` roadmap.
