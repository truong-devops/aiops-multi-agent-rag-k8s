# Data Ownership

Data ownership prevents hidden coupling between services. A service may expose APIs/events for its data, but other services should not read or write its database tables directly.

## Ownership Map

| Data | Owner | Notes |
|---|---|---|
| Public routing policy | `api-gateway` | No product business data. |
| User account | `identity-service` | Auth and profile source of truth. |
| User credentials/session | `identity-service` | Must be protected and never sent to AIOps. |
| Video metadata | `video-service` | Canonical video lifecycle state. |
| Upload request | `video-service` | Includes object key and upload status. |
| Raw/processed objects | MinIO, coordinated by `video-service` and `media-worker` | Object metadata remains in service DBs. |
| Processing job | `media-worker` | Attempts, retry state, error code. |
| Feed items | `feed-social-service` | Can be denormalized from `video.ready`. |
| Likes/comments/follows | `feed-social-service` | Social graph and interactions. |
| Live sessions | `live-service` | Stream key and live status. |
| Incidents/evidence/RCA | `aiops-service` | AIOps-owned operational analysis data. |

## Cross-Service Read Rules

- Prefer API calls for synchronous reads.
- Prefer events for state propagation.
- Do not share database schemas between services.
- Denormalized read models are allowed if they are built from events.
- Admin web should call APIs, not databases.
- External clients should call services through `api-gateway`.

## State That Must Be Explicit

Video state:

```text
draft -> uploaded -> processing -> ready
                         └-------> failed
```

Processing job state:

```text
queued -> running -> succeeded
              └---> retrying -> failed -> dead_letter
```

Live session state:

```text
scheduled -> live -> ended
               └---> failed
```

## AIOps Evidence Requirements

Every important state transition should produce enough evidence for RCA:

- `service`
- `environment`
- `request_id`
- `correlation_id`
- `entity_id` such as `video_id`, `job_id`, `live_session_id`
- `event_name`
- `previous_state`
- `new_state`
- `error_code` when failed
- `timestamp`
