# Packages

Thư mục này chứa contract và tài nguyên dùng chung giữa services/apps.

- `contracts`: OpenAPI, AsyncAPI, JSON Schema, event schema.
- `proto`: protobuf/gRPC definitions nếu dùng.
- `shared-docs`: conventions cấu hình, logging fields, metric names và tài liệu dùng chung.

Không đặt business logic dùng chung quá sớm. Chỉ thêm package chung khi có contract thật hoặc duplication rõ ràng.
