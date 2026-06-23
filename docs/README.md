# Docs

Tài liệu dự án.

- `architecture`: kiến trúc hệ thống, service boundary, repo structure.
- `development`: chuẩn dependency versioning, tooling và workflow phát triển.
- `service`: kế hoạch triển khai chi tiết cho từng service.
- `api`: API docs và contract notes.
- `runbooks`: runbook theo incident type.
- `incidents`: incident dataset, script tái hiện, ground truth.
- `experiments`: kết quả đánh giá RCA.
- `thesis`: tài liệu phục vụ khóa luận.

Roadmap HTML chi tiết được giữ ngoài repo source tại `../Lộ trình triển khai.html` để không push lên GitHub.

## Nên Đọc Theo Thứ Tự

1. [Project Context](../PROJECT_CONTEXT.md)
2. [Project Progress](../PROJECT_PROGRESS.md)
3. [Product Design](./architecture/product-design.md)
4. [Service Boundaries](./architecture/service-boundaries.md)
5. [Data Ownership](./architecture/data-ownership.md)
6. [Database Design](./architecture/database-design.md)
7. [Repo Structure](./architecture/repo-structure.md)
8. [Product Code Rules](./development/product-code-rules.md)
9. [Dependency Versioning](./development/dependency-versioning.md)
10. [RESTful API Design](./api/rest-api-design.md)
11. [Product API Surface](./api/product-api-surface.md)
12. [Event Contracts](../packages/contracts/event-contracts.md)
13. [Identity Service Plan](./service/identity-service-plan.md)
14. [API Gateway Plan](./service/api-gateway-plan.md)
