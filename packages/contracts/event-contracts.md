# Event Contracts

Event contracts are part of the product architecture. They allow services to evolve independently and give the AIOps layer structured evidence for incident analysis.

## Common Envelope

```json
{
  "event_id": "evt_01H...",
  "event_name": "video.uploaded",
  "event_version": "v1",
  "occurred_at": "2026-06-11T10:00:00Z",
  "producer": "video-service",
  "environment": "dev",
  "correlation_id": "corr_01H...",
  "request_id": "req_01H...",
  "payload": {}
}
```

## Video Events

### `video.uploaded.v1`

Published by `video-service` after upload metadata is committed.

```json
{
  "video_id": "vid_01H...",
  "owner_id": "usr_01H...",
  "raw_object_key": "raw/vid_01H/input.mp4",
  "content_type": "video/mp4",
  "size_bytes": 10485760
}
```

### `video.processing_started.v1`

Published by `media-worker` when a processing attempt starts.

```json
{
  "video_id": "vid_01H...",
  "job_id": "job_01H...",
  "attempt": 1,
  "worker_id": "media-worker-abc123"
}
```

### `video.ready.v1`

Published by `video-service` after the canonical video status transitions to `ready`. `media-worker` supplies processed asset metadata through the internal video status API.

```json
{
  "video_id": "vid_01H...",
  "owner_id": "usr_01H...",
  "title": "Demo video",
  "description": "Short description",
  "processed_object_key": "processed/vid_01H/output.mp4",
  "thumbnail_object_key": "thumbs/vid_01H/thumb.jpg",
  "duration_ms": 42000,
  "visibility": "public",
  "ready_at": "2026-06-11T10:03:00Z"
}
```

### `video.processing_failed.v1`

Published by `media-worker` when processing fails permanently or enters dead-letter.

```json
{
  "video_id": "vid_01H...",
  "job_id": "job_01H...",
  "attempt": 3,
  "error_code": "FFMPEG_EXIT_NON_ZERO",
  "retryable": false
}
```

## Social Events

- `video.liked.v1`
- `comment.created.v1`
- `user.followed.v1`

## Live Events

- `live.created.v1`
- `live.started.v1`
- `live.ended.v1`
- `live.failed.v1`

## Contract Rules

- Event names are append-only once consumed by another service.
- Breaking changes require a new `event_version`.
- Payload must not contain secrets, passwords, JWTs or presigned URLs.
- Errors should use stable `error_code` values so RCA can group failures.
