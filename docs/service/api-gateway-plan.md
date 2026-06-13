# API Gateway Implementation Plan

`api-gateway` là entrypoint chính của sản phẩm. Client không nên gọi trực tiếp từng service nội bộ khi sản phẩm bắt đầu có nhiều service. Gateway giúp chuẩn hóa routing, CORS, request ID, rate limit, auth verification và forwarding user context.

## 1. Vai trò

`api-gateway` chịu trách nhiệm:

- Nhận public HTTP traffic từ web/mobile/admin.
- Route request đến service nội bộ.
- Gắn `X-Request-ID` và `X-Correlation-ID`.
- Verify JWT bằng JWKS từ `identity-service` trong giai đoạn sau.
- Forward user context an toàn cho service nội bộ.
- Áp CORS policy.
- Áp rate limiting cho endpoint nhạy cảm.
- Ghi access log phục vụ observability và RCA.

`api-gateway` không chịu trách nhiệm:

- Lưu user/video/feed/live data.
- Tự generate token.
- Xử lý business logic.
- Query database của service khác.
- Tự remediation hoặc apply Kubernetes change.

## 2. Public routing

Public API nên đi qua prefix `/api/v1`.

```text
/api/v1/auth/*          -> identity-service /v1/auth/*
/api/v1/users/*         -> identity-service /v1/users/*
/api/v1/videos/*        -> video-service /v1/videos/*
/api/v1/feed*           -> feed-social-service /v1/feed*
/api/v1/live-sessions/* -> live-service /v1/live-sessions/*
/api/v1/incidents/*     -> aiops-service /v1/incidents/*
```

Health endpoints của gateway:

```text
GET /healthz
GET /readyz
GET /metrics
```

## 3. Deployment position

Trong Kubernetes:

```text
Internet / local browser
-> Nginx Ingress
-> api-gateway
-> internal services
```

Ingress chỉ nên làm TLS, host routing và path forwarding cấp ngoài. Gateway làm policy cấp ứng dụng.

## 4. Auth strategy

Giai đoạn đầu:

- Gateway route request.
- Service tự verify JWT khi cần.
- Dễ debug local.

Giai đoạn sau:

- Gateway fetch JWKS từ `identity-service`.
- Gateway verify JWT.
- Gateway forward internal headers:

```text
X-User-ID
X-User-Email
X-User-Roles
X-Auth-Provider
```

Service nội bộ vẫn nên kiểm tra request đến từ trusted network/gateway.

## 5. Security policies

Gateway nên có:

- CORS allowlist cho admin web/mobile dev origins.
- Rate limit cho login/register/refresh.
- Request body size limit cho API thường.
- Timeout cho upstream requests.
- Header sanitization.
- Access log không ghi token/password.

Không nên log:

```text
Authorization
Cookie
password
refresh_token
authorization_code
presigned_url
```

## 6. Observability

Gateway log fields:

```text
service=api-gateway
request_id
correlation_id
method
path
status
duration_ms
upstream_service
upstream_status
user_id if authenticated
error_code if failed
```

Metrics:

```text
gateway_requests_total
gateway_request_duration_seconds
gateway_upstream_errors_total
gateway_rate_limited_total
gateway_auth_failed_total
```

## 7. Configuration

Environment variables:

```text
PORT=8080
LOG_LEVEL=info
IDENTITY_SERVICE_URL=http://identity-service
VIDEO_SERVICE_URL=http://video-service
FEED_SERVICE_URL=http://feed-social-service
LIVE_SERVICE_URL=http://live-service
AIOPS_SERVICE_URL=http://aiops-service
JWKS_URL=http://identity-service/.well-known/jwks.json
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

## 8. Implementation order

### Step 1 — Basic gateway

- Health/readiness/metrics.
- Static route config by environment variables.
- Reverse proxy to internal services.
- Request ID/correlation ID middleware.
- Access log.

### Step 2 — Product routing

- Route identity APIs.
- Route video APIs.
- Route feed APIs.
- Route live APIs.
- Route AIOps APIs.

### Step 3 — CORS and limits

- CORS allowlist.
- Body size limit.
- Upstream timeout.
- Standard error response for gateway failures.

### Step 4 — JWT verification

- Fetch JWKS from identity-service.
- Cache keys.
- Verify JWT issuer/audience/expiry.
- Forward user context.
- Protect private routes.

### Step 5 — Rate limiting

- Rate limit login/register/refresh.
- Rate limit by IP and optionally user ID.
- Emit metrics for rate-limited requests.

## 9. MVP Definition of Done

Gateway MVP is done when:

- Gateway starts locally.
- `/healthz`, `/readyz`, `/metrics` work.
- Requests to `/api/v1/auth/*` reach `identity-service`.
- Requests to `/api/v1/videos/*` reach `video-service`.
- Request ID and correlation ID are forwarded.
- Docker image builds.
- Kubernetes dev manifest exists.

## 10. Later

- JWT/JWKS verification.
- CORS and rate limit policies.
- OpenTelemetry tracing.
- Per-route timeout/retry policy.
- Admin-visible gateway health panel.
