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

1. [Product Design](./architecture/product-design.md)
2. [Service Boundaries](./architecture/service-boundaries.md)
3. [Data Ownership](./architecture/data-ownership.md)
4. [Database Design](./architecture/database-design.md)
5. [Dependency Versioning](./development/dependency-versioning.md)
6. [RESTful API Design](./api/rest-api-design.md)
7. [Product API Surface](./api/product-api-surface.md)
8. [Event Contracts](../packages/contracts/event-contracts.md)
9. [Identity Service Plan](./service/identity-service-plan.md)
10. [API Gateway Plan](./service/api-gateway-plan.md)
