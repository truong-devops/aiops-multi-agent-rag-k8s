# Product Design

Tài liệu này chốt thiết kế sản phẩm ở mức đủ nghiêm túc để phát triển lâu dài, nhưng vẫn vừa sức cho khóa luận. Sản phẩm chính là một nền tảng video/livestream dạng microservices; phần AIOps dùng chính sản phẩm này làm môi trường sinh incident và thu thập evidence.

## Product Positioning

Sản phẩm không cần cạnh tranh đầy đủ với TikTok/YouTube, nhưng phải có luồng vận hành thật:

- User đăng ký, đăng nhập, có profile.
- Creator upload video.
- Hệ thống xử lý video bất đồng bộ.
- Viewer xem feed video đã sẵn sàng.
- User tương tác cơ bản: like, comment, follow.
- Creator tạo livestream session.
- Admin quan sát video, job xử lý, live session, incident và RCA report.

## Core User Roles

| Role | Mục tiêu chính |
|---|---|
| Viewer | Xem feed, xem video, xem livestream, like/comment/follow. |
| Creator | Upload video, theo dõi trạng thái xử lý, tạo live session. |
| Admin/Ops | Quản lý user/video/live, xem health, incident, RCA report. |
| AIOps System | Thu thập evidence, phân tích incident, đề xuất remediation an toàn. |

## Product Modules

### Identity

Quản lý đăng ký, đăng nhập, profile cơ bản, JWT/session và user context.

Không đưa logic video/feed/live vào service này.

### Video

Quản lý video metadata, upload request, object key, trạng thái xử lý và phát event cho worker.

Video service không chạy FFmpeg trực tiếp. Nó chỉ điều phối upload flow và trạng thái nghiệp vụ.

### Media Processing

Xử lý video bất đồng bộ bằng FFmpeg, tạo thumbnail, cập nhật trạng thái job/video, retry và dead-letter.

Đây là module quan trọng nhất để tạo incident vận hành như OOMKilled, queue lag, FFmpeg error, retry storm.

### Feed & Social

Phục vụ feed video đã publish, like, comment và follow ở mức sản phẩm demo. Giai đoạn đầu giữ chung trong một service để tránh over-engineering.

Sau này có thể tách thành `comment-service`, `relation-service`, `recommendation-service` nếu dữ liệu và traffic đủ lớn.

### Live

Quản lý live session, stream key, trạng thái live và tích hợp MediaMTX.

Giai đoạn đầu chỉ cần live flow đủ demo; không cần full moderation/transcoding/multi-CDN.

### Admin & Operations

Admin web là trung tâm demo sản phẩm và AIOps:

- Users.
- Videos.
- Processing jobs.
- Live sessions.
- Service health.
- Incidents.
- RCA reports.

## Primary Product Flow

```text
User login
-> create video upload request
-> upload file to MinIO
-> video-service stores metadata
-> video-service publishes video.uploaded
-> media-worker consumes event
-> media-worker downloads object
-> FFmpeg processes video and thumbnail
-> media-worker updates video status
-> feed-social-service exposes ready video
-> admin-web shows processing history and health
```

## Livestream Flow

```text
Creator creates live session
-> live-service creates stream key
-> creator streams to MediaMTX
-> live-service tracks session state
-> viewer opens live playback URL
-> admin-web observes live state and latency signals
```

## Extension Points

Không thêm các service này ngay từ đầu, nhưng thiết kế hiện tại để mở rộng được:

| Future Service | Khi nào tách? |
|---|---|
| `notification-service` | Khi cần push/email/in-app notification. |
| `search-service` | Khi feed/query không còn đủ cho tìm kiếm video/user. |
| `recommendation-service` | Khi cần ranking cá nhân hóa thay vì feed đơn giản. |
| `moderation-service` | Khi cần kiểm duyệt video/comment/live. |
| `analytics-service` | Khi cần dashboard product metrics độc lập. |
| `billing-service` | Nếu sau này có monetization. |

## Product-Grade Rules

- Mỗi service sở hữu database/schema của mình; service khác không đọc thẳng DB.
- Giao tiếp đồng bộ dùng HTTP/gRPC qua API rõ ràng.
- Giao tiếp bất đồng bộ dùng event contract rõ ràng.
- Mọi request phải có `request_id` hoặc `trace_id`.
- Mọi job/event phải có `event_id`, `correlation_id`, `video_id` hoặc `job_id`.
- Trạng thái video/job phải explicit, không suy đoán từ log.
- Không tự động sửa production trực tiếp; remediation đi qua GitOps proposal/MR.
- Logs có thể dùng cho AIOps nhưng phải tránh lộ secret, token, presigned URL.

## MVP Product Scope

MVP nên tập trung làm chắc:

1. Identity: register/login/JWT/profile.
2. Video upload: metadata + MinIO object.
3. Media worker: consume event + FFmpeg placeholder/real processing.
4. Feed: list ready videos.
5. Admin: videos, processing jobs, health, incident view.
6. Kubernetes deployment + logs/metrics.

Mobile app và livestream có thể làm sau khi luồng video upload/processing đã ổn.
