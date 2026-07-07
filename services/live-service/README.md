# live-service

Service quan ly live session va stream key cho product app.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trang Thai Hien Tai

Da co production-shaped MVP:

- Config validation va local in-memory fallback.
- PostgreSQL schema/repository cho `live_sessions`, `stream_keys`, `live_events`.
- `POST /v1/live-sessions` tao live session va tra stream key mot lan.
- `GET /v1/live-sessions` list session theo `status`, `creator_id`, `limit`, `cursor`.
- `GET /v1/live-sessions/{live_session_id}` lay chi tiet session.
- `POST /v1/live-sessions/{live_session_id}/start` chuyen `scheduled -> live`.
- `POST /v1/live-sessions/{live_session_id}/end` chuyen `live -> ended`.
- Owner/admin authorization cho start/end thong qua trusted `X-User-*` headers tu gateway.
- `/healthz`, `/readyz`, `/metrics`, structured request/correlation logging.

## API

Public client goi qua gateway:

```text
POST /api/v1/live-sessions
GET  /api/v1/live-sessions
GET  /api/v1/live-sessions/{live_session_id}
POST /api/v1/live-sessions/{live_session_id}/start
POST /api/v1/live-sessions/{live_session_id}/end
```

Service noi bo dung cung resource duoi `/v1`.

Create request:

```json
{
  "title": "Live demo",
  "description": "Kubernetes livestream test",
  "scheduled_at": "2026-07-07T12:00:00Z"
}
```

Create response tra `stream_key` mot lan. Cac API read/list khong tra lai `stream_key`.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP server port. |
| `ENVIRONMENT` | `local` | Runtime environment label. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `DATABASE_URL` | empty | PostgreSQL connection string. Required outside local/dev/test. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | Max request body size. |
| `LIVE_DEFAULT_LIMIT` | `20` | Default list limit. |
| `LIVE_MAX_LIMIT` | `50` | Max list limit. |
| `LIVE_INGEST_BASE_URL` | `rtmp://localhost:1935/live` | Public ingest base URL exposed in API responses. |
| `LIVE_PLAYBACK_BASE_URL` | `http://localhost:8888/live` | Playback base URL exposed in API responses. |
| `STREAM_KEY_BYTES` | `32` | Random byte count before hex encoding the stream key. Minimum `24`. |

## Data Rules

- PostgreSQL is canonical for live lifecycle state.
- Stream key plaintext is returned only during creation and is stored as a hash.
- Redis heartbeat/viewer count and MediaMTX webhook integration are planned later.
- Live chat is out of scope until the core app flow is stable.

## Incident Signals

- Invalid state transitions produce `LIVE_INVALID_STATE`.
- Dependency failures surface through `/readyz`, DB operation metrics and request logs.
- `live_events` records lifecycle events for later AIOps evidence collection.
