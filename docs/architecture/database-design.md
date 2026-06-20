# Database Design

Tài liệu này cố định cách dùng PostgreSQL, MongoDB và Redis cho toàn bộ dự án. Mục tiêu là tránh mỗi service tự chọn DB tùy tiện, đồng thời vẫn đủ linh hoạt để mở rộng sản phẩm video/livestream và AIOps về sau.

## 1. Storage strategy

| Storage | Vai trò trong dự án | Dùng khi | Không dùng khi |
|---|---|---|---|
| PostgreSQL | Source of truth cho dữ liệu giao dịch, trạng thái nghiệp vụ, quan hệ cần ràng buộc | user/session, video lifecycle, upload request, processing job, like/follow, live session | document evidence lớn, cache ngắn hạn, counter realtime |
| MongoDB | Source of truth cho document linh hoạt, evidence pack, RCA report, comment/feed read model | comments, feed snapshots, incidents, evidence, RCA report, agent runs | credential/session, dữ liệu cần unique constraint và transaction chặt |
| Redis | Ephemeral store cho cache, rate limit, distributed lock, counter realtime, idempotency ngắn hạn | gateway rate limit, feed cache, live viewer count, worker lock, AIOps job lock | source of truth nghiệp vụ, dữ liệu cần audit lâu dài |

Quy tắc quan trọng:

- Mỗi service sở hữu database/schema/collection của mình. Service khác không đọc thẳng DB.
- PostgreSQL và MongoDB là durable storage. Redis không được là bản ghi duy nhất của dữ liệu quan trọng.
- Cross-service reference chỉ lưu bằng ID như `user_id`, `video_id`, `job_id`; không tạo foreign key xuyên service.
- Event và API là cách đồng bộ dữ liệu giữa service.
- Dữ liệu trong MongoDB phải có `schema_version` để sau này migrate document an toàn.
- Mọi timestamp lưu dạng UTC. PostgreSQL dùng `timestamptz`; MongoDB dùng `Date`.

## 2. Ownership map

| Service | PostgreSQL | MongoDB | Redis | Ghi chú |
|---|---|---|---|---|
| `api-gateway` | none | none | rate limit, JWKS cache, upstream health cache | Không sở hữu business data. |
| `identity-service` | users, credentials, OAuth accounts, sessions, refresh tokens, auth audit logs | none | login/register rate limit, optional OAuth state cache | PostgreSQL là source of truth cho auth/session. |
| `video-service` | videos, upload requests, video assets, status history, outbox events | none | short-lived upload intent cache, object metadata cache | Video lifecycle phải nằm ở PostgreSQL. |
| `media-worker` | processing jobs, attempts, dead letters | none | distributed job locks, idempotency keys, queue lag cache | Retry/dead-letter cần audit được trong PostgreSQL. |
| `feed-social-service` | likes, follows, social counters | comments, feed items/read model | feed cache, hot counters, dedup keys | MongoDB hợp với comment tree và feed snapshot linh hoạt. |
| `live-service` | live sessions, stream keys, live events | optional live chat messages later | live heartbeats, viewer count, stream health cache | Live session canonical vẫn ở PostgreSQL. |
| `aiops-service` | none for MVP | incidents, evidence items, RCA reports, agent runs, runbook chunks | analysis locks, collector cache, progress cache | MongoDB hợp với evidence/RCA document. Qdrant vẫn dùng riêng cho vector embedding. |

## 3. Physical layout

Local/demo có thể dùng một PostgreSQL cluster, một MongoDB cluster và một Redis instance để tiết kiệm tài nguyên. Tuy vậy vẫn nên tách logical database/schema theo service.

PostgreSQL:

| Service | Database/schema đề xuất |
|---|---|
| `identity-service` | `identity_db` |
| `video-service` | `video_db` |
| `media-worker` | `media_db` |
| `feed-social-service` | `feed_social_db` |
| `live-service` | `live_db` |

MongoDB:

| Service | Database đề xuất |
|---|---|
| `feed-social-service` | `feed_social_docdb` |
| `aiops-service` | `aiops_docdb` |
| `live-service` later | `live_docdb` nếu thêm live chat |

Redis:

Không dựa vào Redis numbered database ở production. Dùng key prefix theo service:

```text
gw:...
identity:...
video:...
media:...
feed:...
live:...
aiops:...
```

## 4. Common field conventions

### IDs

Dùng string ID có prefix để dễ debug log và evidence:

| Entity | Prefix |
|---|---|
| user | `usr_` |
| session | `sess_` |
| refresh token row | `rft_` |
| video | `vid_` |
| upload request | `upl_` |
| asset | `asset_` |
| processing job | `job_` |
| processing attempt | `att_` |
| comment | `cmt_` |
| live session | `live_` |
| incident | `inc_` |
| evidence | `evd_` |
| RCA report | `rca_` |

### Common PostgreSQL columns

```sql
id          text primary key,
created_at  timestamptz not null,
updated_at  timestamptz not null
```

Các bảng có state machine phải có:

```sql
status      text not null,
error_code  text,
deleted_at  timestamptz
```

### Common MongoDB fields

```json
{
  "_id": "cmt_...",
  "schema_version": 1,
  "created_at": "Date",
  "updated_at": "Date",
  "deleted_at": "Date|null"
}
```

### Common operational fields

Các entity quan trọng nên lưu:

```text
request_id
correlation_id
environment
created_by
updated_by
```

Không lưu token, password, authorization code, presigned URL hoặc stream key plaintext.

## 5. identity-service

Primary DB: PostgreSQL.

Redis chỉ dùng cho rate limit hoặc cache ngắn hạn. Không lưu credential/session canonical trong Redis.

### `users`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `usr_...` |
| `email` | `text` | yes | unique, lowercase |
| `username` | `text` | no | unique if not null |
| `display_name` | `text` | yes | default empty string |
| `avatar_url` | `text` | yes | default empty string |
| `status` | `text` | yes | `active`, `disabled` |
| `roles` | `jsonb` | yes | example `["user"]`, `["admin"]` |
| `email_verified` | `boolean` | yes | default `false` |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

Indexes/constraints:

- `unique(email)`
- `unique(username)`
- `status in ('active', 'disabled')`

### `user_credentials`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `user_id` | `text` | yes | PK, references `users(id)` |
| `password_hash` | `text` | yes | hash string includes algorithm/params |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

### `oauth_accounts`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `oauth_...` |
| `user_id` | `text` | yes | references `users(id)` |
| `provider` | `text` | yes | first provider: `google` |
| `provider_user_id` | `text` | yes | Google `sub` |
| `provider_email` | `text` | yes | lowercase |
| `email_verified` | `boolean` | yes | must be true for trusted email login |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

Constraints:

- `unique(provider, provider_user_id)`

### `sessions`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `sess_...` |
| `user_id` | `text` | yes | references `users(id)` |
| `user_agent` | `text` | yes | sanitized |
| `ip_address` | `text` | yes | from gateway/proxy |
| `status` | `text` | yes | `active`, `revoked`, `compromised` |
| `created_at` | `timestamptz` | yes | UTC |
| `last_seen_at` | `timestamptz` | yes | update on refresh |
| `expires_at` | `timestamptz` | yes | refresh session expiry |
| `revoked_at` | `timestamptz` | no | set on logout/reuse |

### `refresh_tokens`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `rft_...` |
| `session_id` | `text` | yes | references `sessions(id)` |
| `token_hash` | `text` | yes | unique, never store raw token |
| `status` | `text` | yes | `active`, `used`, `revoked` |
| `created_at` | `timestamptz` | yes | UTC |
| `expires_at` | `timestamptz` | yes | UTC |
| `used_at` | `timestamptz` | no | set after rotation |
| `revoked_at` | `timestamptz` | no | set on logout/compromise |
| `replaced_by` | `text` | no | next refresh token row ID |

### `oauth_states`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `state` | `text` | yes | PK |
| `provider` | `text` | yes | `google` |
| `nonce` | `text` | yes | OIDC nonce |
| `code_verifier` | `text` | yes | PKCE verifier, server-side only |
| `redirect_uri` | `text` | yes | must match token exchange request |
| `created_at` | `timestamptz` | yes | UTC |
| `expires_at` | `timestamptz` | yes | usually 10 minutes |

### `auth_audit_logs`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `aud_...` |
| `user_id` | `text` | no | nullable for failed login |
| `session_id` | `text` | no | nullable |
| `event_type` | `text` | yes | stable event name |
| `provider` | `text` | yes | empty or `google` |
| `ip_address` | `text` | yes | sanitized |
| `user_agent` | `text` | yes | sanitized |
| `success` | `boolean` | yes | true/false |
| `error_code` | `text` | no | stable error code |
| `created_at` | `timestamptz` | yes | UTC |

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `identity:rl:login:{email_hash}:{ip_hash}` | counter | 5-15m | login rate limit |
| `identity:rl:register:{ip_hash}` | counter | 5-15m | register rate limit |
| `identity:oauth_state:{state}` | string/json | 10m | optional cache; PostgreSQL remains source of truth if implemented |

## 6. video-service

Primary DB: PostgreSQL.

Video service owns canonical video metadata and upload lifecycle. MinIO stores binary objects; PostgreSQL stores object keys and metadata.

### `videos`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `vid_...` |
| `owner_id` | `text` | yes | user ID from identity |
| `title` | `text` | yes | display title |
| `description` | `text` | yes | default empty |
| `status` | `text` | yes | `draft`, `uploaded`, `processing`, `ready`, `failed`, `deleted` |
| `visibility` | `text` | yes | `public`, `private`, `unlisted` |
| `raw_object_key` | `text` | no | MinIO key |
| `processed_object_key` | `text` | no | MinIO key |
| `thumbnail_object_key` | `text` | no | MinIO key |
| `content_type` | `text` | no | e.g. `video/mp4` |
| `size_bytes` | `bigint` | no | raw upload size |
| `duration_ms` | `bigint` | no | set after processing |
| `width` | `integer` | no | set after processing |
| `height` | `integer` | no | set after processing |
| `processing_error_code` | `text` | no | last stable error code |
| `published_at` | `timestamptz` | no | set when ready/public |
| `deleted_at` | `timestamptz` | no | soft delete |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

Indexes:

- `(owner_id, created_at desc)`
- `(status, updated_at desc)`
- `(visibility, published_at desc)`

### `upload_requests`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `upl_...` |
| `video_id` | `text` | yes | references local `videos(id)` |
| `owner_id` | `text` | yes | user ID |
| `bucket` | `text` | yes | MinIO bucket |
| `object_key` | `text` | yes | raw object key |
| `status` | `text` | yes | `created`, `uploaded`, `expired`, `cancelled` |
| `content_type` | `text` | yes | expected content type |
| `size_bytes` | `bigint` | no | expected or final size |
| `checksum_sha256` | `text` | no | optional client checksum |
| `expires_at` | `timestamptz` | yes | presigned URL expiry |
| `completed_at` | `timestamptz` | no | upload confirmed |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

### `video_assets`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `asset_...` |
| `video_id` | `text` | yes | references local `videos(id)` |
| `asset_type` | `text` | yes | `raw`, `mp4_720p`, `hls_master`, `thumbnail` |
| `bucket` | `text` | yes | MinIO bucket |
| `object_key` | `text` | yes | MinIO key |
| `content_type` | `text` | yes | MIME type |
| `size_bytes` | `bigint` | no | object size |
| `width` | `integer` | no | video/image width |
| `height` | `integer` | no | video/image height |
| `duration_ms` | `bigint` | no | video duration |
| `created_at` | `timestamptz` | yes | UTC |

### `video_status_history`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `vsh_...` |
| `video_id` | `text` | yes | local video ID |
| `previous_status` | `text` | no | nullable for first state |
| `new_status` | `text` | yes | target status |
| `reason` | `text` | no | short reason |
| `error_code` | `text` | no | stable code |
| `request_id` | `text` | no | traceability |
| `correlation_id` | `text` | no | traceability |
| `created_at` | `timestamptz` | yes | UTC |

### `outbox_events`

Use this table if the service publishes events to Redpanda/Kafka.

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `evt_...` |
| `event_name` | `text` | yes | e.g. `video.uploaded` |
| `event_version` | `text` | yes | `v1` |
| `aggregate_id` | `text` | yes | `video_id` |
| `payload` | `jsonb` | yes | event payload |
| `status` | `text` | yes | `pending`, `published`, `failed` |
| `published_at` | `timestamptz` | no | UTC |
| `created_at` | `timestamptz` | yes | UTC |

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `video:upload_intent:{upload_request_id}` | string/json | until upload expiry | quick upload lookup |
| `video:object_meta:{video_id}` | string/json | 5-15m | avoid repeated object metadata reads |
| `video:idempotency:{request_id}` | string | 1-24h | prevent duplicate upload request |

## 7. media-worker

Primary DB: PostgreSQL.

The worker can consume from Redpanda/Kafka, but job state and retry audit should remain in PostgreSQL.

### `processing_jobs`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `job_...` |
| `video_id` | `text` | yes | cross-service reference |
| `input_bucket` | `text` | yes | MinIO bucket |
| `input_object_key` | `text` | yes | raw video object |
| `status` | `text` | yes | `queued`, `running`, `retrying`, `succeeded`, `failed`, `dead_letter` |
| `priority` | `integer` | yes | default `0` |
| `attempt_count` | `integer` | yes | current count |
| `max_attempts` | `integer` | yes | default `3` |
| `locked_by` | `text` | no | worker ID |
| `locked_until` | `timestamptz` | no | lease expiry |
| `next_run_at` | `timestamptz` | yes | retry scheduling |
| `started_at` | `timestamptz` | no | first running time |
| `completed_at` | `timestamptz` | no | done time |
| `error_code` | `text` | no | stable error code |
| `error_message` | `text` | no | short sanitized message |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

Indexes:

- `(status, next_run_at, priority desc)`
- `(video_id)`
- `(locked_until)`

### `processing_attempts`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `att_...` |
| `job_id` | `text` | yes | references `processing_jobs(id)` |
| `attempt_no` | `integer` | yes | starts at 1 |
| `worker_id` | `text` | yes | pod/worker identifier |
| `status` | `text` | yes | `running`, `succeeded`, `failed` |
| `ffmpeg_command_hash` | `text` | no | do not store huge command if sensitive |
| `started_at` | `timestamptz` | yes | UTC |
| `finished_at` | `timestamptz` | no | UTC |
| `exit_code` | `integer` | no | FFmpeg exit code |
| `error_code` | `text` | no | stable code |
| `stderr_excerpt` | `text` | no | truncated and redacted |
| `metrics` | `jsonb` | yes | duration, CPU/memory if available |

### `dead_letters`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `dlq_...` |
| `job_id` | `text` | yes | failed job |
| `video_id` | `text` | yes | cross-service reference |
| `reason_code` | `text` | yes | stable reason |
| `payload` | `jsonb` | yes | sanitized event/job context |
| `created_at` | `timestamptz` | yes | UTC |

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `media:lock:job:{job_id}` | string | 1-10m | extra distributed lock |
| `media:idempotency:event:{event_id}` | string | 24h | prevent duplicate event processing |
| `media:queue_lag:{queue_name}` | string/int | 30-60s | quick operational display |

## 8. feed-social-service

Primary DB: PostgreSQL + MongoDB.

PostgreSQL owns relationship/interaction facts that need unique constraints. MongoDB owns flexible documents such as comments and feed read models.

### PostgreSQL: `likes`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `like_...` |
| `video_id` | `text` | yes | cross-service reference |
| `user_id` | `text` | yes | cross-service reference |
| `status` | `text` | yes | `active`, `deleted` |
| `created_at` | `timestamptz` | yes | UTC |
| `deleted_at` | `timestamptz` | no | soft delete |

Constraints:

- `unique(video_id, user_id)` for one active like row pattern, or partial unique index if soft-deleting into multiple rows.

### PostgreSQL: `follows`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `flw_...` |
| `follower_id` | `text` | yes | user who follows |
| `followee_id` | `text` | yes | creator/user being followed |
| `status` | `text` | yes | `active`, `deleted`, `blocked` |
| `created_at` | `timestamptz` | yes | UTC |
| `deleted_at` | `timestamptz` | no | soft delete |

Constraints:

- `unique(follower_id, followee_id)`
- `follower_id <> followee_id`

### PostgreSQL: `video_social_counters`

Counters should be updated from durable interaction writes, not only Redis.

| Field | Type | Required | Notes |
|---|---|---:|---|
| `video_id` | `text` | yes | PK |
| `like_count` | `bigint` | yes | default `0` |
| `comment_count` | `bigint` | yes | default `0` |
| `share_count` | `bigint` | yes | default `0` |
| `updated_at` | `timestamptz` | yes | UTC |

### MongoDB: `comments`

Collection: `feed_social_docdb.comments`

```json
{
  "_id": "cmt_01H...",
  "schema_version": 1,
  "video_id": "vid_01H...",
  "author_user_id": "usr_01H...",
  "parent_comment_id": null,
  "root_comment_id": "cmt_01H...",
  "body": "Nice video",
  "status": "visible",
  "moderation": {
    "state": "approved",
    "reason_code": null,
    "checked_at": null
  },
  "stats": {
    "reply_count": 0,
    "like_count": 0
  },
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "created_at": "Date",
  "updated_at": "Date",
  "deleted_at": null
}
```

Indexes:

- `{ "video_id": 1, "created_at": -1 }`
- `{ "parent_comment_id": 1, "created_at": 1 }`
- `{ "author_user_id": 1, "created_at": -1 }`
- Text index on `body` can be added later if search is needed.

Status values:

```text
visible
hidden
deleted
blocked
```

### MongoDB: `feed_items`

Collection: `feed_social_docdb.feed_items`.

This is a denormalized read model built from `video.ready.v1` and social counters. It can be rebuilt from events and service APIs.

```json
{
  "_id": "feed_vid_01H...",
  "schema_version": 1,
  "video_id": "vid_01H...",
  "owner_id": "usr_01H...",
  "title": "Video title snapshot",
  "description": "Short description snapshot",
  "thumbnail_object_key": "thumbs/vid_01H/thumb.jpg",
  "playback_object_key": "processed/vid_01H/output.mp4",
  "duration_ms": 42000,
  "visibility": "public",
  "status": "active",
  "ready_at": "Date",
  "ranking": {
    "score": 0.0,
    "like_count": 0,
    "comment_count": 0,
    "freshness_score": 0.0
  },
  "created_at": "Date",
  "updated_at": "Date"
}
```

Indexes:

- `{ "status": 1, "ready_at": -1 }`
- `{ "owner_id": 1, "ready_at": -1 }`
- `{ "ranking.score": -1, "ready_at": -1 }`

Do not store presigned URLs in MongoDB. Generate playback URL at API edge or media layer.

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `feed:home:{user_id}` | list/json | 30-120s | personalized/basic feed cache |
| `feed:video:{video_id}:counters` | hash | 30-300s | hot counters cache |
| `feed:dedup:like:{video_id}:{user_id}` | string | 1-10m | duplicate click protection |

## 9. live-service

Primary DB: PostgreSQL.

Redis stores realtime state such as viewer count and stream heartbeat. PostgreSQL stores canonical session state.

### `live_sessions`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `live_...` |
| `creator_id` | `text` | yes | user ID |
| `title` | `text` | yes | display title |
| `description` | `text` | yes | default empty |
| `status` | `text` | yes | `scheduled`, `live`, `ended`, `failed`, `cancelled` |
| `stream_key_hash` | `text` | yes | never store plaintext stream key |
| `ingest_path` | `text` | yes | MediaMTX ingest path |
| `playback_path` | `text` | yes | HLS/WebRTC path |
| `scheduled_at` | `timestamptz` | no | optional |
| `started_at` | `timestamptz` | no | UTC |
| `ended_at` | `timestamptz` | no | UTC |
| `failure_code` | `text` | no | stable failure code |
| `created_at` | `timestamptz` | yes | UTC |
| `updated_at` | `timestamptz` | yes | UTC |

Indexes:

- `(creator_id, created_at desc)`
- `(status, updated_at desc)`

### `stream_keys`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `sk_...` |
| `live_session_id` | `text` | yes | references `live_sessions(id)` |
| `key_hash` | `text` | yes | hash only |
| `status` | `text` | yes | `active`, `rotated`, `revoked` |
| `created_at` | `timestamptz` | yes | UTC |
| `rotated_at` | `timestamptz` | no | UTC |
| `revoked_at` | `timestamptz` | no | UTC |

### `live_events`

| Field | Type | Required | Notes |
|---|---|---:|---|
| `id` | `text` | yes | `levt_...` |
| `live_session_id` | `text` | yes | local live session ID |
| `event_type` | `text` | yes | `created`, `started`, `heartbeat_lost`, `ended`, `failed` |
| `payload` | `jsonb` | yes | sanitized event payload |
| `occurred_at` | `timestamptz` | yes | UTC |

### MongoDB later: `live_chat_messages`

Only add this when live chat is implemented.

```json
{
  "_id": "chat_01H...",
  "schema_version": 1,
  "live_session_id": "live_01H...",
  "author_user_id": "usr_01H...",
  "body": "Hello",
  "status": "visible",
  "moderation": {
    "state": "pending",
    "reason_code": null
  },
  "created_at": "Date",
  "deleted_at": null
}
```

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `live:session:{live_session_id}:heartbeat` | string | 10-30s | stream heartbeat |
| `live:session:{live_session_id}:viewer_count` | counter | refreshed | realtime viewer count |
| `live:session:{live_session_id}:state` | hash/json | 30-60s | realtime status cache |

## 10. aiops-service

Primary DB: MongoDB.

AIOps evidence, agent outputs and RCA reports are document-heavy and evolve quickly. MongoDB is a better fit than forcing everything into many relational tables.

Qdrant remains the vector store for embeddings. MongoDB stores document metadata and source content; Qdrant stores vector index references.

### MongoDB: `incidents`

Collection: `aiops_docdb.incidents`.

```json
{
  "_id": "inc_01H...",
  "schema_version": 1,
  "title": "video-service 5xx after deploy",
  "status": "open",
  "severity": "high",
  "environment": "demo",
  "service": "video-service",
  "source": "manual",
  "time_window": {
    "started_at": "Date",
    "ended_at": "Date|null"
  },
  "entity_refs": {
    "deployment": "video-service",
    "namespace": "aiops-demo",
    "pod": "video-service-abc123",
    "video_id": "vid_01H..."
  },
  "symptoms": [
    {
      "type": "http_5xx_rate",
      "summary": "5xx increased above threshold",
      "observed_at": "Date"
    }
  ],
  "labels": ["deploy", "kubernetes"],
  "created_by": "usr_01H...",
  "created_at": "Date",
  "updated_at": "Date",
  "closed_at": null
}
```

Indexes:

- `{ "status": 1, "created_at": -1 }`
- `{ "service": 1, "created_at": -1 }`
- `{ "environment": 1, "severity": 1, "created_at": -1 }`

### MongoDB: `evidence_items`

Collection: `aiops_docdb.evidence_items`.

```json
{
  "_id": "evd_01H...",
  "schema_version": 1,
  "incident_id": "inc_01H...",
  "collector": "loki",
  "source_type": "log",
  "source_ref": {
    "cluster": "local-k3s",
    "namespace": "aiops-demo",
    "service": "video-service",
    "pod": "video-service-abc123"
  },
  "time_range": {
    "from": "Date",
    "to": "Date"
  },
  "content": {
    "summary": "panic: missing DATABASE_URL",
    "excerpt": "redacted short text",
    "raw_object_key": null
  },
  "redaction": {
    "applied": true,
    "rules": ["token", "secret", "presigned_url"]
  },
  "hash": "sha256:...",
  "embedding_ref": {
    "provider": "qdrant",
    "collection": "aiops_evidence",
    "point_id": "evd_01H..."
  },
  "tags": ["error", "deployment"],
  "created_at": "Date"
}
```

Indexes:

- `{ "incident_id": 1, "created_at": 1 }`
- `{ "collector": 1, "source_type": 1, "created_at": -1 }`
- `{ "hash": 1 }`

### MongoDB: `rca_reports`

Collection: `aiops_docdb.rca_reports`.

```json
{
  "_id": "rca_01H...",
  "schema_version": 1,
  "incident_id": "inc_01H...",
  "status": "completed",
  "summary": "Deployment removed DATABASE_URL secret reference.",
  "root_causes": [
    {
      "rank": 1,
      "cause": "Missing DATABASE_URL in deployment config",
      "confidence": 0.86,
      "evidence_ids": ["evd_01H..."]
    }
  ],
  "timeline": [
    {
      "occurred_at": "Date",
      "description": "New deployment rolled out",
      "evidence_ids": ["evd_01H..."]
    }
  ],
  "recommendations": [
    {
      "type": "gitops_change",
      "risk": "low",
      "description": "Restore DATABASE_URL secret reference",
      "requires_human_approval": true
    }
  ],
  "score": {
    "evidence_coverage": 0.8,
    "confidence": 0.86
  },
  "created_at": "Date",
  "updated_at": "Date"
}
```

Indexes:

- `{ "incident_id": 1, "created_at": -1 }`
- `{ "status": 1, "created_at": -1 }`

### MongoDB: `agent_runs`

Collection: `aiops_docdb.agent_runs`.

```json
{
  "_id": "run_01H...",
  "schema_version": 1,
  "incident_id": "inc_01H...",
  "agent_name": "metric_agent",
  "status": "succeeded",
  "input_evidence_ids": ["evd_01H..."],
  "output": {
    "summary": "Error rate increased after rollout",
    "finding_refs": ["evd_01H..."]
  },
  "token_usage": {
    "prompt_tokens": 1200,
    "completion_tokens": 400,
    "total_tokens": 1600
  },
  "started_at": "Date",
  "finished_at": "Date",
  "error": null
}
```

Indexes:

- `{ "incident_id": 1, "started_at": -1 }`
- `{ "agent_name": 1, "status": 1, "started_at": -1 }`

### MongoDB: `runbook_chunks`

Collection: `aiops_docdb.runbook_chunks`.

```json
{
  "_id": "rbk_01H...",
  "schema_version": 1,
  "source_path": "docs/runbooks/video-processing.md",
  "title": "Video processing retry storm",
  "chunk_index": 0,
  "content": "short chunk text",
  "embedding_ref": {
    "provider": "qdrant",
    "collection": "aiops_runbooks",
    "point_id": "rbk_01H..."
  },
  "tags": ["media-worker", "ffmpeg", "retry"],
  "created_at": "Date",
  "updated_at": "Date"
}
```

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `aiops:lock:incident:{incident_id}` | string | 10-30m | one analysis per incident |
| `aiops:collector_cache:{collector}:{hash}` | string/json | 1-10m | avoid repeated expensive collection |
| `aiops:progress:{incident_id}` | hash/json | 1-24h | UI progress status |
| `aiops:rl:analyze:{user_id}` | counter | 1-10m | protect expensive analysis endpoint |

## 11. api-gateway

No durable database.

### Redis keys

| Key | Type | TTL | Purpose |
|---|---|---:|---|
| `gw:rl:ip:{ip_hash}` | counter | 1-5m | global IP rate limit |
| `gw:rl:user:{user_id}` | counter | 1-5m | authenticated user rate limit |
| `gw:jwks:identity` | string/json | 5-15m | JWKS cache from identity-service |
| `gw:upstream_health:{service}` | string/json | 5-30s | short health cache |

## 12. Data flow decisions

### Video upload and processing

```text
video-service writes PostgreSQL videos/upload_requests
-> publishes video.uploaded
-> media-worker writes PostgreSQL processing_jobs/attempts
-> media-worker updates video-service through API/event
-> feed-social-service builds MongoDB feed_items from video.ready
```

### Feed and social

```text
likes/follows -> PostgreSQL
comments -> MongoDB
feed read model -> MongoDB
feed cache -> Redis
```

### AIOps RCA

```text
incident request -> MongoDB incidents
collectors -> MongoDB evidence_items + Qdrant vectors
agents -> MongoDB agent_runs
RCA output -> MongoDB rca_reports
UI progress/cache -> Redis
```

## 13. Migration and implementation order

1. Keep `identity-service` on PostgreSQL first.
2. Implement `video-service` PostgreSQL schema and outbox table.
3. Implement `media-worker` PostgreSQL jobs/attempts schema.
4. Add Redis for gateway rate limit and media-worker lock.
5. Implement `feed-social-service` PostgreSQL likes/follows and MongoDB comments/feed items.
6. Implement `aiops-service` MongoDB incidents/evidence/RCA collections.
7. Add live-service PostgreSQL session schema and Redis heartbeat/viewer count.

This order keeps the MVP product path stable before adding document-heavy features.

## 14. What not to do

- Do not store JWT, refresh token plaintext, password, OAuth code, stream key plaintext or presigned URL in any DB.
- Do not use MongoDB for `identity-service` credentials/sessions.
- Do not use Redis as the only store for likes, follows, comments, jobs or incidents.
- Do not let AIOps write product service databases directly.
- Do not share one service's tables/collections as an integration API.
