# Product API Surface

This is the initial API surface for the product. It is not a full OpenAPI spec yet; it defines what each service is expected to expose.

Public clients should call APIs through `api-gateway` using `/api/v1/*`. Internal services can keep their own `/v1/*` routes.

## api-gateway

```text
GET /healthz
GET /readyz
GET /metrics

/api/v1/auth/*                             -> identity-service
/api/v1/users/*                            -> identity-service
/api/v1/videos/*                           -> video-service
/api/v1/feed*                              -> feed-social-service
/api/v1/videos/{id}/like|comments|social -> feed-social-service
/api/v1/comments/*                         -> feed-social-service
/api/v1/users/{id}/follow                  -> feed-social-service
/api/v1/live-sessions*                     -> live-service
/api/v1/incidents/*                        -> aiops-service
```

## identity-service

```text
POST /v1/auth/register
POST /v1/auth/login
POST /v1/auth/logout
GET  /v1/users/me
PATCH /v1/users/me
GET  /healthz
GET  /readyz
GET  /metrics
```

## video-service

```text
POST /v1/videos/upload-requests
POST /v1/videos/{video_id}/uploaded
GET  /v1/videos/{video_id}
GET  /v1/videos?owner_id=
PATCH /v1/videos/{video_id}/status
GET  /healthz
GET  /readyz
GET  /metrics
```

## feed-social-service

```text
GET  /v1/feed
PUT  /v1/videos/{video_id}/like
DELETE /v1/videos/{video_id}/like
GET  /v1/videos/{video_id}/social
GET  /v1/videos/{video_id}/comments
POST /v1/videos/{video_id}/comments
DELETE /v1/comments/{comment_id}
PUT  /v1/users/{user_id}/follow
DELETE /v1/users/{user_id}/follow
GET  /healthz
GET  /readyz
GET  /metrics
```

## live-service

```text
POST /v1/live-sessions
GET  /v1/live-sessions
GET  /v1/live-sessions/{live_session_id}
POST /v1/live-sessions/{live_session_id}/start
POST /v1/live-sessions/{live_session_id}/end
GET  /healthz
GET  /readyz
GET  /metrics
```

## aiops-service

```text
POST /v1/incidents
GET  /v1/incidents/{incident_id}
POST /v1/incidents/{incident_id}/analyze
GET  /v1/incidents/{incident_id}/rca-report
GET  /healthz
GET  /readyz
GET  /metrics
```

## API Rules

- Public APIs use `/v1`.
- Internal commands should still be explicit and auditable.
- Errors return stable `error_code`.
- Every request should accept or create `request_id`.
- Admin web calls APIs only; it does not read service databases directly.
