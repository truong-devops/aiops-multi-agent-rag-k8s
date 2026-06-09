# feed-social-service

Service phục vụ feed và tương tác xã hội cơ bản.

## Trách Nhiệm

- Feed video đã publish.
- Like/comment/follow ở mức demo sản phẩm.
- Cache feed đơn giản.

## Dependencies Dự Kiến

- PostgreSQL.
- Redis.

## Incident Có Thể Sinh

- Latency cao do query chậm.
- Redis unavailable.
- Cache stampede.
- 5xx sau deploy version mới.
