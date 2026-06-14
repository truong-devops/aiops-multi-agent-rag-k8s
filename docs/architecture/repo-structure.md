# Repo Structure

Repo này là monorepo cho source code, product apps, AIOps service, tooling và docs. Repo GitOps riêng quản lý trạng thái deploy thật.

## Top-Level Layout

```text
services/      backend services, workers, aiops service
apps/          admin web and mobile app
packages/      contracts and shared conventions
deploy/        local compose, k8s templates, helm templates, scripts
infra/         platform notes and local lab setup
docs/          architecture, runbooks, incidents, experiments, thesis
tests/         e2e, smoke and load tests
tools/         incident injector, log generator, RCA evaluator
```

## Runtime Và Version

Runtime, image nền và dependency policy được cố định tại [Dependency Versioning](../development/dependency-versioning.md).

Baseline hiện tại:

- Go services: Go `1.24`, toolchain `go1.24.13`.
- `aiops-service`: Python `3.12.13`.
- Admin web và mobile app chưa scaffold chính thức; version sẽ được chốt trong tài liệu versioning trước khi thêm dependency.

## Quy Tắc Mở Rộng

- Service mới đặt trong `services/<name>`.
- Client app mới đặt trong `apps/<name>`.
- Contract chung đặt trong `packages/contracts`.
- Không tạo shared library cho business logic nếu chưa có nhu cầu thật.
- Manifest deploy thật sau cùng phải đi qua repo GitOps.
- Tài liệu incident và experiment phải đi cùng implementation để phục vụ đánh giá khóa luận.
