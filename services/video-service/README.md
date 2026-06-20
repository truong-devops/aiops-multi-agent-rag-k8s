# video-service

Service quản lý video metadata, upload flow và event phát sang media worker.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Tạo upload request.
- Lưu video metadata.
- Tích hợp MinIO/presigned URL.
- Publish event `video.uploaded`.
- Cập nhật trạng thái video.

## Dependencies Dự Kiến

- PostgreSQL for video metadata, upload requests, assets and outbox events.
- Redis for short-lived upload intent cache and idempotency keys.
- MinIO.
- Redpanda/Kafka.

## Incident Có Thể Sinh

- MinIO AccessDenied.
- Publish event fail.
- DB connection pool cạn.
- Deploy thiếu env/secret.
