# Service Boundaries

Thiết kế service theo bounded context. Mỗi service có quyền sở hữu dữ liệu rõ ràng, API rõ ràng và incident surface rõ ràng để phục vụ AIOps/RCA.

## Boundary Summary

| Service | Owns | Exposes | Emits | Does Not Own |
|---|---|---|---|---|
| `api-gateway` | routing/policy config, request context | public API entrypoint | access logs/metrics | product business data |
| `identity-service` | users, credentials, sessions, profiles | auth/profile APIs | `user.created`, `user.updated` | video, feed, live data |
| `video-service` | video metadata, upload requests, video status | video/upload/status APIs | `video.uploaded`, `video.status_changed` | FFmpeg processing |
| `media-worker` | processing jobs, attempts, worker status | health/metrics, optional job admin APIs | `video.processing_started`, `video.ready`, `video.processing_failed` | user-facing video metadata ownership |
| `feed-social-service` | feed view, likes, comments, follows | feed/social APIs | `video.liked`, `comment.created`, `user.followed` | video upload/processing |
| `live-service` | live sessions, stream keys, live status | live session APIs | `live.started`, `live.ended`, `live.failed` | media processing pipeline |
| `aiops-service` | incidents, evidence packs, RCA reports | incident/RCA APIs | `rca.completed`, `remediation.proposed` | product business data |

## api-gateway

Responsibilities:

- Route public API traffic to internal services.
- Attach `request_id` and `correlation_id`.
- Apply CORS, body limit, timeout and rate limit policies.
- Verify JWT using JWKS from `identity-service` later.
- Forward user context to internal services after verification.

Data ownership:

- No product database.
- Only routing/policy config and operational logs/metrics.

Must not:

- Store user/video/feed/live data.
- Generate platform tokens.
- Implement product business logic.
- Query service databases directly.

## identity-service

Responsibilities:

- Register/login/logout.
- Password hashing and credential management.
- JWT/session issuing.
- Basic profile.
- Auth middleware support through token claims/JWKS later.

Data ownership:

- `users`
- `user_credentials`
- `sessions`
- `profiles`

Must not:

- Store video objects.
- Store feed/social data.
- Call AIOps agents directly.

## video-service

Responsibilities:

- Create upload request.
- Generate object key and upload intent.
- Store video metadata.
- Track video lifecycle: `draft`, `uploaded`, `processing`, `ready`, `failed`, `deleted`.
- Publish video lifecycle events.

Data ownership:

- `videos`
- `video_assets`
- `upload_requests`

Must not:

- Run FFmpeg.
- Own worker retry logic.
- Write feed/social tables directly.

## media-worker

Responsibilities:

- Consume `video.uploaded`.
- Download raw object from MinIO.
- Run FFmpeg or processing placeholder.
- Generate thumbnail/processed outputs.
- Track attempts, retry and dead-letter.
- Update video status through video-service API or a controlled command/event.

Data ownership:

- `processing_jobs`
- `processing_attempts`

Must not:

- Expose public video APIs.
- Own the canonical video metadata.
- Silently mutate video state without an auditable event/API call.

## feed-social-service

Responsibilities:

- Serve feed of ready videos.
- Manage likes, comments and follows.
- Cache feed results.
- Consume video-ready events if denormalized feed tables are needed.

Data ownership:

- `likes`
- `comments`
- `follows`
- `feed_items` if denormalized

Must not:

- Process video files.
- Own upload status.
- Become a recommendation engine too early.

## live-service

Responsibilities:

- Create live sessions.
- Generate stream keys.
- Track live lifecycle: `scheduled`, `live`, `ended`, `failed`.
- Integrate with MediaMTX.

Data ownership:

- `live_sessions`
- `stream_keys`
- `live_events`

Must not:

- Handle long-form video processing.
- Own feed ranking.

## aiops-service

Responsibilities:

- Receive incident context.
- Collect evidence from Kubernetes, Loki, Prometheus, Argo CD, GitLab, Harbor/Trivy and runbooks.
- Redact sensitive data.
- Build evidence packs.
- Run Log, Metric, Deployment and Planner agents.
- Generate RCA report and safe remediation proposal.

Data ownership:

- `incidents`
- `evidence_items`
- `rca_reports`
- `agent_runs`

Must not:

- Apply changes directly to Kubernetes.
- Write product databases directly.
- Claim root cause without evidence references.
