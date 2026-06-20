# live-service

Service quản lý live session và tích hợp MediaMTX.

Runtime: Go `1.24`, toolchain `go1.24.13`, Docker builder `golang:1.24.13-alpine3.23`.

## Trách Nhiệm

- Tạo live session.
- Quản lý stream key.
- Theo dõi trạng thái live.
- Tích hợp MediaMTX.

## Dependencies Dự Kiến

- PostgreSQL for live sessions, stream keys and live events.
- Redis for live heartbeat, viewer count and stream state cache.
- MongoDB later only if live chat messages are implemented.
- MediaMTX.

## Incident Có Thể Sinh

- Livestream latency cao.
- MediaMTX quá tải.
- Sai cấu hình protocol.
- Redis/session lỗi.
