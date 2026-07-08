# docker-compose

Local compose dung de chay product stack gan voi moi truong that nhung van phuc vu development nhanh.

Mac dinh `docker-compose.yml` chay:

- PostgreSQL plus migration job cho `identity_db`, `video_db`, `media_db`, `feed_social_db`, `live_db`.
- Redis.
- MinIO plus bucket init job cho `raw-videos`, `processed-videos`, `thumbnails`.
- Redpanda/Kafka.
- Qdrant.
- MediaMTX.
- `identity-service`, `video-service`, `media-worker`, `feed-social-service`, `live-service`, `api-gateway`.

`aiops-service` nam trong profile rieng vi chua phai product app path chinh.

Kubernetes vẫn là mục tiêu triển khai chính. Compose chỉ phục vụ development nhanh.

Validate compose:

```bash
make compose-config
```

Run product stack:

```bash
make compose-up
```

Run with AIOps profile:

```bash
make compose-up-aiops
```

Stop:

```bash
make compose-down
```

Follow logs:

```bash
make compose-logs
```

Important local notes:

- PostgreSQL database creation scripts under `deploy/docker-compose/postgres-init` only run when the `postgres_data` volume is created the first time. If database names change, run `docker compose down -v` before starting again.
- `identity-service` uses `ENVIRONMENT=local` by default in compose so it can generate an ephemeral JWT key without committing a development private key. It still uses PostgreSQL and Redis because `DATABASE_URL` and `REDIS_URL` are set.
- `video-service` uses `VIDEO_MINIO_ENDPOINT=localhost:9000` by default so presigned upload URLs are usable from the host browser. If `VERIFY_UPLOAD_OBJECT=true`, set `VIDEO_MINIO_ENDPOINT=minio:9000` or provide networking that lets the service reach the same endpoint it signs.
- `media-worker` defaults to `PROCESSING_MODE=placeholder`. Set `PROCESSING_MODE=ffmpeg` when you want real FFmpeg processing.

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
