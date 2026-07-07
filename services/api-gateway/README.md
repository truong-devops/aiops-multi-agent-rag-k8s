# api-gateway

Product edge service for routing public API requests to internal services.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

Initial responsibilities:

- Route `/api/v1/auth/*` and `/api/v1/users/*` to `identity-service`.
- Route `/api/v1/videos/*` to `video-service`.
- Route `/api/v1/feed*` to `feed-social-service`.
- Route social subresources `/api/v1/videos/{video_id}/like|comments|social`, `/api/v1/comments/*`, and `/api/v1/users/{user_id}/follow` to `feed-social-service`.
- Route `/api/v1/live-sessions*` to `live-service`.
- Route `/api/v1/incidents/*` to `aiops-service`.
- Attach `X-Request-ID` and `X-Correlation-ID`.
- Verify JWT access tokens through JWKS from `identity-service` for protected API prefixes.
- Strip client-supplied internal user headers and forward trusted user context after token verification.
- Apply CORS policy, request body limit and upstream response-header timeout.
- Provide `/healthz`, `/readyz`, and `/metrics`.

Planned responsibilities:

- Rate limiting.
- Richer Prometheus metrics with route/upstream labels.
- Upstream health cache.

The gateway should not own business data or implement product business logic.

## Storage

- No durable database.
- Redis is used for rate limiting, JWKS cache and short-lived upstream health cache when those features are enabled.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP server port. |
| `ENVIRONMENT` | `local` | Runtime environment label. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `IDENTITY_SERVICE_URL` | `http://localhost:8081` | Upstream identity service base URL. |
| `VIDEO_SERVICE_URL` | `http://localhost:8082` | Upstream video service base URL. |
| `FEED_SERVICE_URL` | `http://localhost:8083` | Upstream feed-social service base URL. |
| `LIVE_SERVICE_URL` | `http://localhost:8084` | Upstream live service base URL. |
| `AIOPS_SERVICE_URL` | `http://localhost:8085` | Upstream AIOps service base URL. |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | Comma-separated allowed origins. |
| `REQUEST_BODY_LIMIT_BYTES` | `1048576` | Max request body size accepted by gateway. |
| `UPSTREAM_TIMEOUT` | `15s` | Upstream response-header timeout and JWKS fetch timeout. |
| `JWT_VERIFY_ENABLED` | `true` | Enable gateway access-token verification. |
| `JWKS_URL` | `${IDENTITY_SERVICE_URL}/.well-known/jwks.json` | JWKS endpoint used to verify access tokens. |
| `JWT_ISSUER` | `aiops-video-platform` | Expected JWT issuer. |
| `JWT_AUDIENCE` | `aiops-api` | Expected JWT audience. |
| `JWKS_CACHE_TTL` | `5m` | Local JWKS cache duration. |
| `AUTH_REQUIRED_PREFIXES` | `/api/v1/users,/api/v1/videos,/api/v1/comments,/api/v1/live-sessions,/api/v1/incidents` | Comma-separated public API prefixes that require a valid access token. |

When `JWT_VERIFY_ENABLED=true`, `/readyz` checks that JWKS can be fetched. For local-only development without `identity-service`, set `JWT_VERIFY_ENABLED=false`.
