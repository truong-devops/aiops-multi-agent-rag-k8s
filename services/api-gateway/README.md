# api-gateway

Product edge service for routing public API requests to internal services.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

Initial responsibilities:

- Route `/api/v1/auth/*` and `/api/v1/users/*` to `identity-service`.
- Route `/api/v1/videos/*` to `video-service`.
- Route `/api/v1/feed*` to `feed-social-service`.
- Route `/api/v1/live-sessions/*` to `live-service`.
- Route `/api/v1/incidents/*` to `aiops-service`.
- Attach `X-Request-ID` and `X-Correlation-ID`.
- Provide `/healthz`, `/readyz`, and `/metrics`.

Planned responsibilities:

- JWT verification through JWKS from `identity-service`.
- Rate limiting.
- CORS policy.
- Request/response access logs.
- Internal user context forwarding.

The gateway should not own business data or implement product business logic.

## Storage

- No durable database.
- Redis is used for rate limiting, JWKS cache and short-lived upstream health cache when those features are enabled.
