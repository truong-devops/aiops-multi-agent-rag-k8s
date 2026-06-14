# identity-service

Auth service cho nền tảng video/livestream.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Đăng ký, đăng nhập.
- Quản lý profile cơ bản.
- Phát hành JWT/session.
- Cung cấp user context cho các service khác.

## Dependencies Dự Kiến

- PostgreSQL.
- Redis.

## Incident Có Thể Sinh

- Thiếu `DATABASE_URL`.
- Redis timeout.
- Sai JWT secret.
- Connection pool cạn.
