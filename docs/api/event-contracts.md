# Event Contracts

Tai lieu nay mo ta cac event bat dong bo can on dinh schema de cac service va AI agent doc lai khong phai suy doan.

## Envelope Chung

Moi event nen dung envelope:

```json
{
  "event_id": "evt_...",
  "event_name": "video.ready",
  "event_version": "v1",
  "event_type": "video.ready.v1",
  "aggregate_id": "vid_...",
  "producer": "video-service",
  "environment": "local",
  "correlation_id": "corr_...",
  "request_id": "req_...",
  "occurred_at": "2026-07-03T10:00:00Z",
  "payload": {}
}
```

Rules:

- `event_type` is the stable routing key: `{event_name}.{event_version}`.
- `aggregate_id` is the canonical entity id, usually `video_id`.
- Payloads must include evidence fields needed for incident investigation.
- Services must consume event versions explicitly; do not silently accept incompatible payloads.

## `video.processing_started.v1`

Current producer decision:

- Canonical lifecycle publishing remains owned by `video-service`.
- `media-worker` currently triggers this lifecycle by calling the internal video status API with status `processing`.
- `media-worker` has contract builders for this event, but does not publish it directly yet.

Payload:

```json
{
  "video_id": "vid_...",
  "owner_id": "usr_...",
  "job_id": "job_...",
  "attempt_id": "att_...",
  "worker_id": "media-worker-0",
  "status": "running"
}
```

## `video.ready.v1`

Current producer decision:

- Canonical lifecycle publishing remains owned by `video-service`.
- `media-worker` triggers readiness by calling the internal video status API with status `ready`.
- Direct `media-worker` publishing is deferred until a downstream service requires worker-owned output metadata events.

Payload:

```json
{
  "video_id": "vid_...",
  "owner_id": "usr_...",
  "job_id": "job_...",
  "attempt_id": "att_...",
  "worker_id": "media-worker-0",
  "status": "succeeded",
  "processed_object_key": "processed/vid_.../source.mp4",
  "thumbnail_object_key": "thumbnails/vid_.../poster.jpg",
  "duration_ms": 12340,
  "width": 1280,
  "height": 720,
  "size_bytes": 4096
}
```

## `video.processing_failed.v1`

Current producer decision:

- Canonical lifecycle publishing remains owned by `video-service`.
- `media-worker` triggers final failure by calling the internal video status API with status `failed` after retry exhaustion or permanent failures.

Payload:

```json
{
  "video_id": "vid_...",
  "owner_id": "usr_...",
  "job_id": "job_...",
  "attempt_id": "att_...",
  "worker_id": "media-worker-0",
  "status": "failed",
  "error_code": "FFMPEG_FAILED"
}
```

## Deferred Direct Publishing

If `media-worker` later publishes lifecycle events directly, add:

- `outbox_events` table owned by `media-worker`.
- Transactional outbox write beside attempt success/failure state changes.
- Publisher worker with retry/backoff and publish metrics.
- Consumer contract tests in downstream services.

Do not add direct publishing just to duplicate `video-service` lifecycle events.
