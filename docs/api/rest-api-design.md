# RESTful API Design

Tài liệu này định nghĩa chuẩn REST API cho AIOps video platform. Public client sẽ gọi qua `api-gateway`; các service nội bộ vẫn giữ route `/v1/*` của riêng mình.

## 1. API Gateway Convention

Public API:

```text
/api/v1/*
```

Internal service API:

```text
/v1/*
```

Gateway mapping:

```text
/api/v1/auth/*          -> identity-service /v1/auth/*
/api/v1/users/*         -> identity-service /v1/users/*
/api/v1/videos/*        -> video-service /v1/videos/*
/api/v1/feed*           -> feed-social-service /v1/feed*
/api/v1/videos/{id}/like|comments|social -> feed-social-service /v1/videos/{id}/like|comments|social
/api/v1/comments/*      -> feed-social-service /v1/comments/*
/api/v1/users/{id}/follow -> feed-social-service /v1/users/{id}/follow
/api/v1/live-sessions*  -> live-service /v1/live-sessions*
/api/v1/incidents/*     -> aiops-service /v1/incidents/*
```

Client không gọi trực tiếp service nội bộ khi chạy như sản phẩm.

## 2. REST Naming Rules

Use:

```text
Plural nouns: /videos, /users, /comments
Kebab-case path names: /upload-requests, /live-sessions
Snake_case JSON fields: video_id, created_at, error_code
Version prefix: /v1
```

Avoid:

```text
/getVideo
/createVideo
/doLogin
/updateStatusVideo
```

Command-like endpoints are allowed only when the action is not naturally represented as a resource update, for example:

```text
POST /v1/auth/login
POST /v1/auth/refresh
POST /v1/incidents/{incident_id}/analysis-runs
```

## 3. Standard Headers

Request headers:

```text
Authorization: Bearer <access_token>
Content-Type: application/json
Accept: application/json
X-Request-ID: req_...
X-Correlation-ID: corr_...
Idempotency-Key: idem_...
```

Gateway-internal headers:

```text
X-User-ID: usr_...
X-User-Email: user@example.com
X-User-Roles: user,admin
X-Gateway: api-gateway
```

Rules:

- `X-Request-ID` identifies one HTTP request.
- `X-Correlation-ID` groups multiple operations in one workflow.
- `Idempotency-Key` should be supported for non-idempotent `POST` endpoints such as upload request creation.
- Do not log `Authorization`, `Cookie`, refresh token, password, OAuth code or presigned URL.

## 4. Response Envelope

Success response:

```json
{
  "data": {},
  "request_id": "req_01HX..."
}
```

List response:

```json
{
  "data": [],
  "page": {
    "limit": 20,
    "next_cursor": "cursor_01HX...",
    "has_more": true
  },
  "request_id": "req_01HX..."
}
```

Error response:

```json
{
  "error": {
    "code": "VIDEO_NOT_FOUND",
    "message": "Video was not found.",
    "details": {}
  },
  "request_id": "req_01HX..."
}
```

## 5. HTTP Status Codes

| Code | Meaning | Usage |
|---|---|---|
| `200` | OK | Read/update success. |
| `201` | Created | Resource created. |
| `202` | Accepted | Async work accepted. |
| `204` | No Content | Delete/unlike/logout success. |
| `400` | Bad Request | Invalid payload or query. |
| `401` | Unauthorized | Missing/invalid token. |
| `403` | Forbidden | Authenticated but not allowed. |
| `404` | Not Found | Resource does not exist or is hidden. |
| `409` | Conflict | Duplicate or invalid state transition. |
| `422` | Unprocessable Entity | Valid JSON but invalid domain rule. |
| `429` | Too Many Requests | Rate limited. |
| `500` | Internal Server Error | Unexpected service error. |
| `502` | Bad Gateway | Gateway upstream failed. |
| `503` | Service Unavailable | Dependency unavailable. |

## 6. Pagination, Filtering, Sorting

Cursor pagination:

```text
GET /api/v1/videos?limit=20&cursor=cursor_...
```

Filtering:

```text
GET /api/v1/videos?owner_id=usr_...&status=ready
GET /api/v1/live-sessions?status=live
GET /api/v1/incidents?service=video-service&status=open
```

Sorting:

```text
GET /api/v1/videos?sort=-created_at
GET /api/v1/comments?sort=created_at
```

Rules:

- Default `limit`: 20.
- Max `limit`: 100.
- Sort prefix `-` means descending.
- Cursor pagination is preferred over offset pagination.

## 7. Identity API

### Register

```text
POST /api/v1/auth/register
```

Request:

```json
{
  "email": "user@example.com",
  "username": "user01",
  "display_name": "User One",
  "password": "strong-password"
}
```

Response `201`:

```json
{
  "data": {
    "user": {
      "id": "usr_01HX...",
      "email": "user@example.com",
      "username": "user01",
      "display_name": "User One",
      "status": "active",
      "created_at": "2026-06-13T10:00:00Z"
    }
  },
  "request_id": "req_01HX..."
}
```

Errors:

```text
EMAIL_ALREADY_EXISTS
USERNAME_ALREADY_EXISTS
WEAK_PASSWORD
VALIDATION_ERROR
```

### Login

```text
POST /api/v1/auth/login
```

Request:

```json
{
  "email": "user@example.com",
  "password": "strong-password"
}
```

Response `200`:

```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "rt_...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "usr_01HX...",
      "email": "user@example.com",
      "display_name": "User One",
      "roles": ["user"]
    }
  },
  "request_id": "req_01HX..."
}
```

Errors:

```text
INVALID_CREDENTIALS
USER_DISABLED
RATE_LIMITED
```

### Refresh Token

```text
POST /api/v1/auth/refresh
```

Request:

```json
{
  "refresh_token": "rt_..."
}
```

Response `200`:

```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "rt_new...",
    "token_type": "Bearer",
    "expires_in": 900
  },
  "request_id": "req_01HX..."
}
```

Errors:

```text
INVALID_REFRESH_TOKEN
REFRESH_TOKEN_REUSED
SESSION_REVOKED
```

### Logout

```text
POST /api/v1/auth/logout
```

Request:

```json
{
  "refresh_token": "rt_..."
}
```

Response:

```text
204 No Content
```

### Google OAuth Start

```text
GET /api/v1/auth/google/start?redirect_uri=http://localhost:3000/auth/callback
```

Response `200`:

```json
{
  "data": {
    "authorization_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "state": "state_..."
  },
  "request_id": "req_01HX..."
}
```

### Google OAuth Token Exchange

```text
POST /api/v1/auth/google/token
```

Request:

```json
{
  "code": "google_authorization_code",
  "state": "state_...",
  "code_verifier": "pkce_verifier",
  "redirect_uri": "http://localhost:3000/auth/callback"
}
```

Response is the same shape as login.

Errors:

```text
GOOGLE_STATE_INVALID
GOOGLE_TOKEN_EXCHANGE_FAILED
GOOGLE_ID_TOKEN_INVALID
GOOGLE_EMAIL_NOT_VERIFIED
```

### Current User

```text
GET /api/v1/users/me
PATCH /api/v1/users/me
```

Patch request:

```json
{
  "display_name": "New Name",
  "avatar_url": "https://example.com/avatar.png"
}
```

## 8. Video API

### Create Upload Request

```text
POST /api/v1/videos/upload-requests
```

Headers:

```text
Authorization: Bearer <access_token>
Idempotency-Key: idem_...
```

Request:

```json
{
  "title": "My first video",
  "description": "Demo upload",
  "content_type": "video/mp4",
  "size_bytes": 10485760
}
```

Response `201`:

```json
{
  "data": {
    "video_id": "vid_01HX...",
    "upload_request_id": "upl_01HX...",
    "status": "draft",
    "object_key": "raw/vid_01HX/input.mp4",
    "upload": {
      "method": "PUT",
      "url": "https://minio.example.com/...",
      "expires_at": "2026-06-13T10:15:00Z"
    }
  },
  "request_id": "req_01HX..."
}
```

Errors:

```text
UNSUPPORTED_CONTENT_TYPE
VIDEO_TOO_LARGE
UPLOAD_REQUEST_EXISTS
MINIO_UNAVAILABLE
```

### Mark Upload Completed

```text
POST /api/v1/videos/{video_id}/upload-completions
```

Request:

```json
{
  "upload_request_id": "upl_01HX...",
  "object_key": "raw/vid_01HX/input.mp4",
  "checksum_sha256": "..."
}
```

Response `202`:

```json
{
  "data": {
    "video_id": "vid_01HX...",
    "status": "uploaded",
    "event": "video.uploaded.v1"
  },
  "request_id": "req_01HX..."
}
```

### Get Video

```text
GET /api/v1/videos/{video_id}
```

Response:

```json
{
  "data": {
    "id": "vid_01HX...",
    "owner_id": "usr_01HX...",
    "title": "My first video",
    "description": "Demo upload",
    "status": "ready",
    "thumbnail_url": "https://...",
    "playback_url": "https://...",
    "created_at": "2026-06-13T10:00:00Z",
    "updated_at": "2026-06-13T10:05:00Z"
  },
  "request_id": "req_01HX..."
}
```

### List Videos

```text
GET /api/v1/videos?owner_id=usr_...&status=ready&limit=20&cursor=...
```

### Update Video Metadata

```text
PATCH /api/v1/videos/{video_id}
```

Request:

```json
{
  "title": "Updated title",
  "description": "Updated description"
}
```

### Delete Video

```text
DELETE /api/v1/videos/{video_id}
```

Response:

```text
204 No Content
```

## 9. Media Processing Admin API

These APIs can be exposed later for admin/debug only.

```text
GET /api/v1/videos/{video_id}/processing-jobs
GET /api/v1/processing-jobs/{job_id}
POST /api/v1/processing-jobs/{job_id}/retries
```

Retry response should be `202 Accepted`.

## 10. Feed and Social API

### Feed

```text
GET /api/v1/feed?limit=20&cursor=...
```

Response:

```json
{
  "data": [
    {
      "video_id": "vid_01HX...",
      "owner": {
        "id": "usr_01HX...",
        "display_name": "Creator"
      },
      "title": "My first video",
      "description": "Short description",
      "thumbnail_object_key": "thumbnails/vid_01HX/poster.jpg",
      "playback_object_key": "processed/vid_01HX/source.mp4",
      "duration_ms": 12340,
      "like_count": 10,
      "comment_count": 2,
      "ready_at": "2026-06-13T10:00:00Z"
    }
  ],
  "page": {
    "limit": 20,
    "next_cursor": "cursor_...",
    "has_more": true
  },
  "request_id": "req_01HX..."
}
```

### Like Video

```text
PUT /api/v1/videos/{video_id}/like
DELETE /api/v1/videos/{video_id}/like
```

Use `PUT` because liking is idempotent.

Response:

```json
{
  "data": {
    "video_id": "vid_01HX...",
    "like_count": 11,
    "comment_count": 2,
    "share_count": 0
  },
  "liked": true,
  "changed": true,
  "request_id": "req_01HX..."
}
```

### Social Counters

```text
GET /api/v1/videos/{video_id}/social
```

### Comments

```text
GET  /api/v1/videos/{video_id}/comments?limit=20&cursor=...
POST /api/v1/videos/{video_id}/comments
DELETE /api/v1/comments/{comment_id}
```

Create comment request:

```json
{
  "body": "Nice video!"
}
```

Comment response bodies should not expose deleted or hidden comment text.

### Follow User

```text
PUT /api/v1/users/{user_id}/follow
DELETE /api/v1/users/{user_id}/follow
```

Use `PUT` because following is idempotent.

## 11. Live API

### Create Live Session

```text
POST /api/v1/live-sessions
```

Request:

```json
{
  "title": "Live demo",
  "description": "Kubernetes livestream test",
  "scheduled_at": "2026-06-13T12:00:00Z"
}
```

Response `201`:

```json
{
  "data": {
    "id": "liv_01HX...",
    "owner_id": "usr_01HX...",
    "status": "scheduled",
    "stream_key": "sk_...",
    "ingest_url": "rtmp://media.example.com/live",
    "playback_url": "https://media.example.com/live/liv_01HX/index.m3u8"
  },
  "request_id": "req_01HX..."
}
```

### Live Session Lifecycle

```text
GET  /api/v1/live-sessions/{live_session_id}
POST /api/v1/live-sessions/{live_session_id}/start
POST /api/v1/live-sessions/{live_session_id}/end
```

Lifecycle endpoints return `202 Accepted` if the state change is asynchronous.

### List Live Sessions

```text
GET /api/v1/live-sessions?status=live&limit=20&cursor=...
```

## 12. AIOps API

### Create Incident

```text
POST /api/v1/incidents
```

Request:

```json
{
  "service": "video-service",
  "namespace": "app-demo",
  "symptom": "CrashLoopBackOff",
  "severity": "high",
  "started_at": "2026-06-13T10:00:00Z",
  "time_window": "30m"
}
```

Response `201`:

```json
{
  "data": {
    "id": "inc_01HX...",
    "status": "open",
    "service": "video-service",
    "symptom": "CrashLoopBackOff"
  },
  "request_id": "req_01HX..."
}
```

### Get Incident

```text
GET /api/v1/incidents/{incident_id}
```

### Start RCA Analysis

```text
POST /api/v1/incidents/{incident_id}/analysis-runs
```

Response `202`:

```json
{
  "data": {
    "analysis_run_id": "run_01HX...",
    "incident_id": "inc_01HX...",
    "status": "queued"
  },
  "request_id": "req_01HX..."
}
```

### Get RCA Report

```text
GET /api/v1/incidents/{incident_id}/rca-report
```

Response:

```json
{
  "data": {
    "incident_id": "inc_01HX...",
    "root_cause_candidates": [
      {
        "cause": "DATABASE_URL was removed from deployment config.",
        "confidence": 0.87,
        "evidence_ids": ["ev_01HX...", "ev_01HY..."]
      }
    ],
    "recommended_actions": [
      {
        "type": "gitops_patch",
        "description": "Restore DATABASE_URL secret reference."
      }
    ]
  },
  "request_id": "req_01HX..."
}
```

## 13. Admin API Notes

Admin web should use the same public API through `api-gateway`. It should not read databases directly.

Admin-only endpoints can be added later under:

```text
/api/v1/admin/*
```

Do not add admin endpoints until the user-facing APIs and service ownership are stable.

## 14. Error Code Catalog

Common:

```text
VALIDATION_ERROR
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
CONFLICT
RATE_LIMITED
UPSTREAM_UNAVAILABLE
INTERNAL_ERROR
```

Identity:

```text
EMAIL_ALREADY_EXISTS
USERNAME_ALREADY_EXISTS
INVALID_CREDENTIALS
INVALID_REFRESH_TOKEN
REFRESH_TOKEN_REUSED
GOOGLE_TOKEN_EXCHANGE_FAILED
GOOGLE_ID_TOKEN_INVALID
```

Video:

```text
VIDEO_NOT_FOUND
VIDEO_INVALID_STATE
UPLOAD_REQUEST_NOT_FOUND
UNSUPPORTED_CONTENT_TYPE
VIDEO_TOO_LARGE
MINIO_UNAVAILABLE
EVENT_PUBLISH_FAILED
```

Media:

```text
PROCESSING_JOB_NOT_FOUND
PROCESSING_JOB_NOT_RETRYABLE
FFMPEG_FAILED
OBJECT_DOWNLOAD_FAILED
DEAD_LETTERED
```

Live:

```text
LIVE_SESSION_NOT_FOUND
LIVE_INVALID_STATE
STREAM_KEY_INVALID
MEDIAMTX_UNAVAILABLE
```

AIOps:

```text
INCIDENT_NOT_FOUND
INCIDENT_INVALID_STATE
EVIDENCE_COLLECTION_FAILED
RCA_ANALYSIS_FAILED
RCA_REPORT_NOT_READY
```

## 15. REST Design Decisions

- Use `PUT` for idempotent like/follow operations.
- Use `POST` for commands that create sub-resources or start async workflows.
- Use `PATCH` for partial resource updates.
- Use `DELETE` for soft-delete or relationship removal.
- Use `202 Accepted` when processing happens asynchronously.
- Use stable error codes so the admin UI and AIOps service can group failures.
- Keep public API through gateway. Internal services can use the same resource design with `/v1`.
