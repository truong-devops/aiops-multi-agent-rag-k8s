# video-service

Service quản lý video metadata, upload flow và event phát sang media worker.

## Trách Nhiệm

- Tạo upload request.
- Lưu video metadata.
- Tích hợp MinIO/presigned URL.
- Publish event `video.uploaded`.
- Cập nhật trạng thái video.

## Dependencies Dự Kiến

- PostgreSQL.
- MinIO.
- Redpanda/Kafka.

## Incident Có Thể Sinh

- MinIO AccessDenied.
- Publish event fail.
- DB connection pool cạn.
- Deploy thiếu env/secret.
