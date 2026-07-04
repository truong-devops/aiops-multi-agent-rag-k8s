# docker-compose

Local compose dùng để chạy nhanh dependencies:

- PostgreSQL.
- PostgreSQL test profile for `video-service` and `media-worker` integration tests.
- Redis.
- MinIO.
- Redpanda/Kafka.
- Qdrant.
- MediaMTX nếu cần.

Kubernetes vẫn là mục tiêu triển khai chính. Compose chỉ phục vụ development nhanh.

Run the disposable `video-service` PostgreSQL test database and integration tests with:

```bash
make test-video-integration
```

Run the disposable `media-worker` PostgreSQL test database and integration tests with:

```bash
make test-media-integration
```

Run the local FFmpeg processor smoke test with:

```bash
make smoke-media-ffmpeg
```
