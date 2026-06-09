# media-worker

Worker xử lý video bất đồng bộ bằng FFmpeg.

## Trách Nhiệm

- Consume event `video.uploaded`.
- Tải file từ MinIO.
- Chạy FFmpeg.
- Tạo thumbnail.
- Cập nhật trạng thái `pending`, `processing`, `ready`, `failed`.
- Retry và dead-letter queue.

## Dependencies Dự Kiến

- Redpanda/Kafka.
- MinIO.
- PostgreSQL.
- FFmpeg.

## Incident Có Thể Sinh

- OOMKilled khi xử lý video lớn.
- Queue lag tăng.
- FFmpeg lỗi.
- Retry storm.
- MinIO latency cao.
