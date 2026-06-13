# Identity Service Implementation Plan

`identity-service` là service đầu tiên nên triển khai vì nó cung cấp user identity, authentication context và token cho toàn bộ platform. Các service sau như `video-service`, `feed-social-service`, `live-service`, `admin-web` đều cần biết request đến từ user nào.

Mục tiêu là làm đủ chuẩn để mở rộng về sau, nhưng vẫn triển khai được theo MVP.

## 1. Vai trò của identity-service

`identity-service` chịu trách nhiệm:

- Đăng ký tài khoản bằng email/password.
- Đăng nhập bằng email/password.
- Đăng nhập bằng Google OAuth/OIDC.
- Phát hành access token JWT.
- Quản lý refresh token/session.
- Cung cấp profile cơ bản.
- Expose JWKS để API Gateway hoặc service khác verify JWT.
- Ghi audit log cho login/logout/token refresh.

`identity-service` không chịu trách nhiệm:

- Upload video.
- Feed/social interaction.
- Livestream.
- Authorization nghiệp vụ phức tạp của từng service.
- AIOps/RCA analysis.

## 2. Có cần API Gateway không?

Có. Với định hướng làm sản phẩm hoàn chỉnh, nên có `api-gateway` như entrypoint chính ngay từ đầu, nhưng triển khai theo từng mức.

Giai đoạn đầu gateway chỉ cần route, gắn request ID/correlation ID và chuẩn hóa public path. Nginx Ingress vẫn đứng ngoài để làm TLS/host routing:

```text
Client
-> Nginx Ingress / Kubernetes Ingress
-> api-gateway
-> internal services
```

API Gateway nên làm:

- TLS termination.
- Route request theo path.
- CORS.
- Rate limiting.
- Request ID/correlation ID.
- Verify JWT bằng JWKS từ `identity-service`.
- Forward user context qua headers nội bộ.

API Gateway không nên làm:

- Lưu user database.
- Xử lý login business logic.
- Tự generate JWT.
- Gọi database của service khác.
- Chứa business rule của video/feed/live.

### Gateway routing đề xuất

```text
/api/v1/auth/*          -> identity-service
/api/v1/users/*         -> identity-service
/api/v1/videos/*        -> video-service
/api/v1/feed*           -> feed-social-service
/api/v1/live-sessions/* -> live-service
/api/v1/incidents/*     -> aiops-service
```

### Internal headers sau khi verify JWT

Gateway có thể forward:

```text
X-Request-ID
X-Correlation-ID
X-User-ID
X-User-Email
X-User-Roles
```

Các service vẫn nên có khả năng verify JWT trực tiếp trong giai đoạn đầu để đơn giản hóa local development. Khi API Gateway ổn định, việc verify có thể chuyển dần về gateway, còn service chỉ trust internal network + signed headers nếu cần.

## 3. Auth strategy

Nên hỗ trợ 2 loại đăng nhập:

1. Email/password.
2. Google OAuth/OIDC.

Sau khi xác thực thành công, dù user login bằng email/password hay Google, hệ thống vẫn phát hành token nội bộ của platform:

```text
platform access token: JWT
platform refresh token: opaque random token
```

Không dùng Google token để gọi các service nội bộ. Google chỉ là identity provider để xác minh người dùng.

## 4. Token model

### Access token

Access token nên là JWT, thời gian sống ngắn.

Đề xuất:

```text
TTL: 15 minutes
Algorithm: RS256 hoặc EdDSA
Issuer: aiops-video-platform
Audience: aiops-api
```

MVP có thể dùng HS256 để làm nhanh, nhưng bản chuẩn nên dùng asymmetric key và JWKS để gateway/service verify mà không cần giữ private key.

Claims đề xuất:

```json
{
  "iss": "aiops-video-platform",
  "aud": "aiops-api",
  "sub": "usr_01HX...",
  "email": "user@example.com",
  "roles": ["user"],
  "sid": "sess_01HX...",
  "jti": "jwt_01HX...",
  "iat": 1718000000,
  "nbf": 1718000000,
  "exp": 1718000900
}
```

### Refresh token

Refresh token không nên là JWT. Nên là opaque random token, lưu hash trong DB.

Đề xuất:

```text
TTL: 7-30 days
Storage client: httpOnly secure cookie cho web, secure storage cho mobile
Storage server: hash only
Rotation: enabled
Reuse detection: enabled
```

Khi refresh:

```text
old refresh token -> validate hash -> revoke old token -> issue new refresh token -> issue new access token
```

Nếu token cũ đã bị revoke mà vẫn được dùng lại, coi là dấu hiệu token theft và revoke toàn bộ session.

## 5. Google OAuth/OIDC flow

Nên dùng Authorization Code Flow with PKCE.

### Flow tổng quát

```text
1. Client gọi identity-service để bắt đầu Google login.
2. identity-service tạo state, nonce, code_verifier/code_challenge.
3. Client redirect user sang Google authorization endpoint.
4. Google redirect về callback với authorization code.
5. identity-service exchange code lấy token từ Google.
6. identity-service validate ID token.
7. identity-service tìm hoặc tạo user.
8. identity-service tạo session.
9. identity-service phát hành platform access token + refresh token.
```

### API đề xuất

```text
GET  /v1/auth/google/start
GET  /v1/auth/google/callback
POST /v1/auth/google/token
```

Có 2 cách triển khai:

#### Web redirect flow

Phù hợp admin web/browser:

```text
GET /v1/auth/google/start
-> redirect to Google
-> GET /v1/auth/google/callback?code=...&state=...
-> set refresh token cookie
-> redirect về admin-web
```

#### Token exchange flow

Phù hợp mobile app:

```text
mobile app nhận authorization code
-> POST /v1/auth/google/token
-> identity-service exchange code và trả platform tokens
```

### ID token validation bắt buộc

Khi nhận ID token từ Google, phải validate:

- `iss` đúng issuer của Google.
- `aud` đúng Google client ID của mình.
- `exp` chưa hết hạn.
- `iat` hợp lý.
- `nonce` khớp nếu dùng redirect flow.
- `email_verified = true` nếu dùng email làm identifier tin cậy.

Không tin email từ client gửi lên trực tiếp.

## 6. Database design

Nên dùng PostgreSQL.

### users

```sql
users (
  id              text primary key,
  email           text not null unique,
  username        text unique,
  display_name    text,
  avatar_url      text,
  status          text not null,
  email_verified  boolean not null default false,
  created_at      timestamptz not null,
  updated_at      timestamptz not null
)
```

Status đề xuất:

```text
active
disabled
deleted
```

### user_credentials

Chỉ dùng cho email/password users.

```sql
user_credentials (
  user_id         text primary key references users(id),
  password_hash   text not null,
  password_algo   text not null,
  created_at      timestamptz not null,
  updated_at      timestamptz not null
)
```

Password hashing:

```text
Argon2id preferred
bcrypt acceptable for MVP
```

### oauth_accounts

Dùng để link Google account với user nội bộ.

```sql
oauth_accounts (
  id                text primary key,
  user_id           text not null references users(id),
  provider          text not null,
  provider_user_id  text not null,
  provider_email    text not null,
  email_verified    boolean not null,
  created_at        timestamptz not null,
  updated_at        timestamptz not null,
  unique(provider, provider_user_id)
)
```

Provider ban đầu:

```text
google
```

### sessions

```sql
sessions (
  id             text primary key,
  user_id        text not null references users(id),
  user_agent     text,
  ip_address     text,
  status         text not null,
  created_at     timestamptz not null,
  last_seen_at   timestamptz,
  expires_at     timestamptz not null,
  revoked_at     timestamptz
)
```

Session status:

```text
active
revoked
expired
compromised
```

### refresh_tokens

```sql
refresh_tokens (
  id             text primary key,
  session_id     text not null references sessions(id),
  token_hash     text not null unique,
  status         text not null,
  created_at     timestamptz not null,
  expires_at     timestamptz not null,
  used_at        timestamptz,
  revoked_at     timestamptz,
  replaced_by    text
)
```

### auth_audit_logs

```sql
auth_audit_logs (
  id             text primary key,
  user_id        text,
  session_id     text,
  event_type     text not null,
  provider       text,
  ip_address     text,
  user_agent     text,
  success        boolean not null,
  error_code     text,
  created_at     timestamptz not null
)
```

Event types:

```text
user.registered
auth.login_succeeded
auth.login_failed
auth.google_login_succeeded
auth.google_login_failed
auth.token_refreshed
auth.logout
auth.refresh_reuse_detected
```

## 7. API design

### Password auth

```text
POST /v1/auth/register
POST /v1/auth/login
POST /v1/auth/logout
POST /v1/auth/refresh
```

### Google auth

```text
GET  /v1/auth/google/start
GET  /v1/auth/google/callback
POST /v1/auth/google/token
```

### User profile

```text
GET   /v1/users/me
PATCH /v1/users/me
```

### Key discovery

```text
GET /.well-known/jwks.json
```

### Health and metrics

```text
GET /healthz
GET /readyz
GET /metrics
```

## 8. Response shape

### Success

```json
{
  "data": {
    "access_token": "...",
    "expires_in": 900,
    "token_type": "Bearer",
    "user": {
      "id": "usr_01HX...",
      "email": "user@example.com",
      "display_name": "User"
    }
  },
  "request_id": "req_01HX..."
}
```

### Error

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Invalid email or password."
  },
  "request_id": "req_01HX..."
}
```

Stable error codes:

```text
EMAIL_ALREADY_EXISTS
INVALID_CREDENTIALS
USER_DISABLED
INVALID_REFRESH_TOKEN
REFRESH_TOKEN_REUSED
GOOGLE_STATE_INVALID
GOOGLE_TOKEN_EXCHANGE_FAILED
GOOGLE_ID_TOKEN_INVALID
GOOGLE_EMAIL_NOT_VERIFIED
```

## 9. Security requirements

- Password must be hashed, never encrypted/plaintext.
- Refresh token must be stored as hash only.
- Access token TTL must be short.
- Refresh token rotation must be enabled.
- Google OAuth must validate state and nonce.
- JWT private key must not be shared with other services.
- JWT verification should use public JWKS.
- Login failures should use generic error messages.
- Rate limit login/register/refresh endpoints at gateway level.
- Do not log passwords, tokens, authorization codes or presigned URLs.

## 10. Observability requirements

Every log should include:

```text
service
environment
level
request_id
correlation_id
user_id if known
event
error_code if failed
```

Metrics:

```text
http_requests_total
http_request_duration_seconds
auth_login_total
auth_login_failed_total
auth_google_login_total
auth_token_refresh_total
auth_refresh_reuse_detected_total
active_sessions
```

Useful traces/spans:

```text
auth.register
auth.login
auth.refresh
auth.google.start
auth.google.callback
db.query
jwt.sign
```

## 11. Events emitted by identity-service

```text
user.created.v1
user.updated.v1
auth.login_succeeded.v1
auth.login_failed.v1
auth.logout.v1
auth.refresh_reuse_detected.v1
```

Do not emit sensitive payloads.

Example:

```json
{
  "event_id": "evt_01HX...",
  "event_name": "user.created",
  "event_version": "v1",
  "producer": "identity-service",
  "occurred_at": "2026-06-12T10:00:00Z",
  "correlation_id": "corr_01HX...",
  "payload": {
    "user_id": "usr_01HX...",
    "email_verified": true,
    "provider": "google"
  }
}
```

## 12. Implementation order

### Step 1 — Foundation

- Config loader.
- Logger.
- HTTP router.
- Error response middleware.
- Request ID middleware.
- PostgreSQL connection.
- Migration runner or migration command.

### Step 2 — User registration

- Create `users`.
- Create `user_credentials`.
- Hash password.
- Return user profile.
- Emit audit log.

### Step 3 — Password login

- Find user by email.
- Verify password.
- Create session.
- Issue access token.
- Issue refresh token.
- Store refresh token hash.

### Step 4 — Token refresh

- Validate refresh token hash.
- Detect reuse.
- Rotate refresh token.
- Issue new access token.
- Update session last seen.

### Step 5 — Logout

- Revoke session.
- Revoke active refresh token.
- Return success even if token already revoked.

### Step 6 — Profile

- `GET /v1/users/me`.
- `PATCH /v1/users/me`.

### Step 7 — Google OAuth/OIDC

- Configure Google client ID/secret/redirect URL.
- Implement start endpoint.
- Store state/nonce temporarily.
- Implement callback/token exchange.
- Validate Google ID token.
- Upsert `users`.
- Upsert `oauth_accounts`.
- Create session and issue platform tokens.

### Step 8 — JWKS

- Use asymmetric signing key.
- Expose `/.well-known/jwks.json`.
- Add `kid` to JWT header.
- Prepare for key rotation later.

### Step 9 — Tests

- Register success.
- Register duplicate email.
- Login success.
- Login invalid password.
- Refresh success.
- Refresh token reuse detection.
- Logout success.
- Google callback invalid state.
- Google ID token invalid.

## 13. MVP vs later

### MVP

- Email/password register/login.
- JWT access token.
- Refresh token rotation.
- Basic profile.
- Health/readiness/metrics.
- Basic audit logs.

### Next

- Google OAuth/OIDC.
- JWKS.
- Rate limiting at gateway.
- Session management UI.

### Later

- RBAC/permissions.
- Email verification flow.
- Password reset.
- MFA.
- Multiple OAuth providers.
- Account linking UI.

## 14. Definition of Done

Identity MVP is done when:

- User can register with email/password.
- User can login and receive access token + refresh token.
- Access token can be verified by another service or gateway.
- User can call `GET /v1/users/me`.
- Refresh token can rotate safely.
- Logout revokes session.
- Logs and metrics are available.
- Database migrations are repeatable.
- Docker image builds.
- Basic tests pass.

Google OAuth is done when:

- User can login with Google through Authorization Code Flow with PKCE.
- ID token is validated server-side.
- Google account is linked to internal user.
- Platform tokens are issued after Google login.
- Failed OAuth attempts produce stable error codes and audit logs.
