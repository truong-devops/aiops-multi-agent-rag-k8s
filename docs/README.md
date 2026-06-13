# Docs

Tài liệu dự án.

- `architecture`: kiến trúc hệ thống, service boundary, repo structure.
- `service`: kế hoạch triển khai chi tiết cho từng service.
- `api`: API docs và contract notes.
- `runbooks`: runbook theo incident type.
- `incidents`: incident dataset, script tái hiện, ground truth.
- `experiments`: kết quả đánh giá RCA.
- `thesis`: tài liệu phục vụ khóa luận.

Roadmap HTML chi tiết được giữ ngoài repo source tại `../lo_trinh_trien_khai_aiops_multiagent_rag.html` để không push lên GitHub.

## Nên Đọc Theo Thứ Tự

1. [Product Design](./architecture/product-design.md)
2. [Service Boundaries](./architecture/service-boundaries.md)
3. [Data Ownership](./architecture/data-ownership.md)
4. [Product API Surface](./api/product-api-surface.md)
5. [Event Contracts](../packages/contracts/event-contracts.md)
6. [Identity Service Plan](./service/identity-service-plan.md)
7. [API Gateway Plan](./service/api-gateway-plan.md)
