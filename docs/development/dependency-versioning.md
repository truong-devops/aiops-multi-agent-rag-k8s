# Dependency Versioning

Tài liệu này là version baseline dùng chung cho toàn bộ source repo. Mục tiêu là tránh tình trạng mỗi service dùng một phiên bản runtime, thư viện hoặc image khác nhau dẫn đến lỗi lệch môi trường giữa local, CI và Kubernetes.

## 1. Quy tắc chung

- Mỗi runtime, framework, thư viện trực tiếp và container image quan trọng phải có version rõ ràng.
- Không dùng `latest`, `*`, floating range hoặc dependency không có version trong code production.
- Direct dependency phải pin exact version khi đưa vào module.
- Transitive dependency phải được khóa bằng lockfile phù hợp với hệ sinh thái của module.
- Nâng version phải đi qua một commit/MR riêng, có ghi lý do và chạy test tối thiểu của module liên quan.
- Khi đổi version trong manifest như `go.mod`, `pyproject.toml`, `package.json`, `pubspec.yaml` hoặc Dockerfile, phải cập nhật lại tài liệu này trong cùng thay đổi.

## 2. Runtime compatibility baseline

Runtime/toolchain được chốt theo compatibility track và bản patch đang dùng được pin rõ trong manifest/image. Patch runtime có thể cập nhật để nhận security fix, nhưng phải đi qua thay đổi có kiểm thử; không đổi major/minor tùy tiện.

| Area | Compatibility track | Nơi áp dụng |
|---|---:|---|
| Go services | Go `1.24`, toolchain `go1.24.13` | `services/*/go.mod`, Go builder image |
| Python AIOps service | Python `3.12.13` | `services/aiops-service/pyproject.toml`, `.python-version`, Python runtime image |
| Admin web | Node.js `22.x` | áp dụng khi scaffold `apps/admin-web` |
| Mobile app | Flutter `3.x` | áp dụng khi scaffold `apps/mobile-flutter` |
| Docker build | Docker `27` | GitLab CI build job |
| Kubernetes local/demo | Kubernetes `1.31+` | k3s/k8s manifests |

Ghi chú: dependency thư viện vẫn phải pin exact ở manifest của từng module. Runtime image trong repo này dùng patch tag cụ thể để tránh image trôi ngầm. Khi đi vào demo ổn định, image production có thể pin thêm digest.

## 3. Lockfile policy

| Ecosystem | Manifest | Lockfile bắt buộc |
|---|---|---|
| Go | `go.mod` | `go.sum` |
| Python | `pyproject.toml` | `uv.lock` hoặc generated lockfile tương đương khi package manager workflow được chốt |
| Node.js | `package.json` | `package-lock.json` |
| Flutter | `pubspec.yaml` | `pubspec.lock` |
| Helm | `Chart.yaml`, `values.yaml` | chart version phải pin trong GitOps repo |

Nếu module chưa có dependency bên ngoài thì lockfile có thể chưa tồn tại. Với Go/Node/Flutter, ngay khi thêm dependency đầu tiên thì lockfile phải được commit. Với Python, hiện tại `aiops-service` đã pin exact direct dependencies trong `pyproject.toml`; `uv.lock` sẽ được thêm khi repo chốt dùng `uv` hoặc một workflow lockfile tương đương.

## 4. Current module version catalog

### Go services

Các Go service hiện tại đang dùng Go `1.24` với toolchain `go1.24.13`. Direct dependencies phải pin exact version trong từng `go.mod`.

| Module | Runtime | External libraries |
|---|---:|---|
| `services/api-gateway` | Go `1.24`, toolchain `go1.24.13` | none |
| `services/identity-service` | Go `1.24`, toolchain `go1.24.13` | `github.com/jackc/pgx/v5 v5.8.0`, `github.com/redis/go-redis/v9 v9.20.1` |
| `services/video-service` | Go `1.24`, toolchain `go1.24.13` | `github.com/jackc/pgx/v5 v5.8.0`, `github.com/segmentio/kafka-go v0.4.51` |
| `services/feed-social-service` | Go `1.24`, toolchain `go1.24.13` | `github.com/jackc/pgx/v5 v5.8.0`, `github.com/segmentio/kafka-go v0.4.51` |
| `services/live-service` | Go `1.24`, toolchain `go1.24.13` | none |
| `services/media-worker` | Go `1.24`, toolchain `go1.24.13` | `github.com/jackc/pgx/v5 v5.8.0`, `github.com/segmentio/kafka-go v0.4.51` |

Khi bắt đầu thêm thư viện Go, dùng dạng exact module version:

```bash
go get github.com/example/module@v1.2.3
go mod tidy
```

### AIOps service

`services/aiops-service` dùng Python `3.12.13`.

Runtime:

| File | Version |
|---|---:|
| `services/aiops-service/.python-version` | `3.12.13` |
| `services/aiops-service/pyproject.toml` | `>=3.12.13,<3.13` |
| `services/aiops-service/Dockerfile` | `python:3.12.13-slim-bookworm` |

| Package | Version | Vai trò |
|---|---:|---|
| `fastapi` | `0.115.0` | HTTP API framework |
| `uvicorn[standard]` | `0.34.0` | ASGI server |
| `pydantic` | `2.10.0` | schema validation |
| `httpx` | `0.28.0` | outbound HTTP client |
| `kubernetes` | `32.0.0` | Kubernetes API collector |
| `prometheus-client` | `0.21.0` | metrics endpoint |
| `qdrant-client` | `1.13.0` | vector database client |
| `pytest` | `8.3.0` | test runner |
| `ruff` | `0.9.0` | lint/format |

Direct dependencies trong `pyproject.toml` phải dùng `==`. Không dùng `>=` cho production dependency.

### Local infrastructure images

| Component | Image |
|---|---|
| PostgreSQL | `postgres:16-alpine` |
| Redis | `redis:7-alpine` |
| MinIO | `minio/minio:RELEASE.2025-04-22T22-12-26Z` |
| Redpanda | `redpandadata/redpanda:v24.3.5` |
| Qdrant | `qdrant/qdrant:v1.14.1` |
| MediaMTX | `bluenviron/mediamtx:1.12.3` |

### CI tool image baseline

Các image CI phải được pin trước khi pipeline dùng cho demo chính thức. Nếu đổi image, cần chạy thử pipeline hoặc job tương ứng.

| Job concern | Image baseline |
|---|---|
| Go validation | `golang:1.24.13-bookworm` |
| Python validation | `python:3.12.13-slim-bookworm` |
| Manifest validation | `registry.k8s.io/kubectl:v1.31.4` |
| Docker build | `docker:27`, `docker:27-dind` |
| Secret scan | `zricethezav/gitleaks:v8.30.1` |
| Filesystem vulnerability scan | `aquasec/trivy:0.71.1` |
| SBOM generation | `anchore/syft:v1.45.1` |
| GitOps update helper | `alpine/git:2.52.0` |

Ghi chú: trước khi đổi `validate:manifests` sang `registry.k8s.io/kubectl:v1.31.4`, cần kiểm tra image có shell/entrypoint phù hợp với GitLab runner.

## 5. Planned shared library baseline

Các thư viện dưới đây là baseline đề xuất khi service bắt đầu triển khai thật. Nếu chọn thư viện khác, cập nhật bảng này trước khi thêm code.

| Concern | Go baseline |
|---|---|
| HTTP router | ưu tiên standard library, chỉ thêm router nếu route phức tạp |
| PostgreSQL client | `github.com/jackc/pgx/v5` với exact version |
| Redis client | `github.com/redis/go-redis/v9` với exact version |
| JWT | `github.com/golang-jwt/jwt/v5` với exact version |
| Password hashing | `golang.org/x/crypto` với exact version |
| Metrics | `github.com/prometheus/client_golang` với exact version |
| Testing assertions | `github.com/stretchr/testify` với exact version |

Không thêm thư viện chỉ vì tiện. Với service Go nhỏ, ưu tiên standard library trước để giảm dependency surface.

## 6. Upgrade checklist

Trước khi nâng version:

1. Đọc changelog/release notes của dependency chính.
2. Cập nhật manifest và lockfile.
3. Chạy unit test của module liên quan.
4. Build Docker image nếu dependency ảnh hưởng runtime.
5. Cập nhật tài liệu này.
6. Ghi rõ lý do nâng version trong commit message hoặc MR description.

## 7. Commit message gợi ý

```text
docs: add dependency versioning baseline
chore(aiops): pin python dependencies
chore(identity): add pinned jwt dependency
```
