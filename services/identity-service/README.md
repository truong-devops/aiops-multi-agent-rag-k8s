# identity-service

Auth service cho nền tảng video/livestream.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

Current implementation runs without external infrastructure by using an in-memory repository. PostgreSQL persistence and Redis-backed session/cache can be added behind the existing repository interfaces later.

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
| `ACCESS_TOKEN_TTL` | `15m` | JWT access token lifetime. |
| `REFRESH_TOKEN_TTL` | `168h` | Refresh token/session lifetime. |
| `GOOGLE_CLIENT_ID` | empty | Google OAuth client ID. |
| `GOOGLE_CLIENT_SECRET` | empty | Google OAuth client secret. |
| `GOOGLE_AUTH_URL` | Google default | Override for tests/local OAuth provider. |
| `GOOGLE_TOKEN_URL` | Google default | Override for tests/local OAuth provider. |
| `GOOGLE_JWKS_URL` | Google default | JWKS endpoint for ID token validation. |

If `SIGNING_KEY_PEM` is not set, the service generates an RSA key on startup. That is fine for local development, but existing access tokens become invalid after restart.

## Dependencies Dự Kiến

- PostgreSQL persistence behind repository interfaces.
- Redis for distributed session/cache concerns if needed later.

## Incident Có Thể Sinh

- Thiếu `DATABASE_URL`.
- Redis timeout.
- Sai JWT secret.
- Connection pool cạn.
