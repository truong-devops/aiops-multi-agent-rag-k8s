# identity-service

Auth service cho nền tảng video/livestream.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

PostgreSQL is the production source of truth for users, credentials, OAuth accounts, sessions, refresh tokens and audit logs. Redis is used for login/register rate limiting. The in-memory repository and no-op rate limiter are kept only for local development and tests when external URLs are not set.

## Trách Nhiệm

- Đăng ký, đăng nhập.
- Quản lý profile cơ bản.
- Phát hành JWT/session.
- Cung cấp user context cho các service khác.
- Expose JWKS for API Gateway and internal service verification.
- Start Google OAuth/OIDC flow when Google client configuration is provided.

## API

Direct service paths:

- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `GET /v1/auth/google/start`
- `GET /v1/auth/google/callback`
- `POST /v1/auth/google/token`
- `GET /v1/users/me`
- `PATCH /v1/users/me`
- `GET /.well-known/jwks.json`
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

Public paths should be reached through `api-gateway` as `/api/v1/...`.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP server port. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `ENVIRONMENT` | `local` | Runtime environment label. |
| `JWT_ISSUER` | `aiops-video-platform` | Access token issuer. |
| `JWT_AUDIENCE` | `aiops-api` | Access token audience. |
| `SIGNING_KEY_PEM` | generated at startup | RSA private key for stable JWT signing. |
| `DATABASE_URL` | empty | PostgreSQL connection string. Required outside local/dev/test. |
| `REDIS_URL` | empty | Redis connection string for auth rate limiting. Required outside local/dev/test. |
| `TRUST_PROXY_HEADERS` | `false` | Trust `X-Forwarded-For`/`X-Real-IP` for rate limiting and audit IPs. Enable only when requests can arrive only through a trusted proxy/gateway. |
| `ACCESS_TOKEN_TTL` | `15m` | JWT access token lifetime. |
| `REFRESH_TOKEN_TTL` | `168h` | Refresh token/session lifetime. |
| `LOGIN_RATE_LIMIT` | `5` | Login attempts allowed per email/IP window. |
| `LOGIN_RATE_LIMIT_WINDOW` | `15m` | Login rate-limit window. |
| `REGISTER_RATE_LIMIT` | `10` | Register attempts allowed per IP window. |
| `REGISTER_RATE_LIMIT_WINDOW` | `15m` | Register rate-limit window. |
| `GOOGLE_CLIENT_ID` | empty | Google OAuth client ID. |
| `GOOGLE_CLIENT_SECRET` | empty | Google OAuth client secret. |
| `GOOGLE_AUTH_URL` | Google default | Override for tests/local OAuth provider. |
| `GOOGLE_TOKEN_URL` | Google default | Override for tests/local OAuth provider. |
| `GOOGLE_JWKS_URL` | Google default | JWKS endpoint for ID token validation. |
| `GOOGLE_ALLOWED_REDIRECT_URIS` | empty | Comma-separated OAuth redirect URI allowlist. Required outside local/dev/test when Google OAuth is enabled. |

If `SIGNING_KEY_PEM` is not set, the service generates an RSA key on startup. That is fine for local development, but existing access tokens become invalid after restart.

For `production`, `staging`, and other non-local environments, startup fails when `DATABASE_URL`, `REDIS_URL` or `SIGNING_KEY_PEM` is missing.

## Database

Apply migrations before starting the service:

```bash
psql "$DATABASE_URL" -f migrations/001_identity_schema.sql
```

The schema stores users, password credentials, OAuth identities, sessions, refresh-token rotation state, OAuth PKCE state, and authentication audit logs.

## Dependencies

- PostgreSQL via `github.com/jackc/pgx/v5`.
- Redis via `github.com/redis/go-redis/v9`.
- MongoDB is not used by `identity-service`.

## Incident Có Thể Sinh

- Thiếu `DATABASE_URL`.
- Thiếu hoặc sai `REDIS_URL`.
- Sai JWT secret.
- Connection pool cạn.
- Redis unavailable làm auth endpoint fail closed.
