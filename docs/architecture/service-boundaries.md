# Service Boundaries

## Product Services

`identity-service` quản lý user và auth. Các service khác chỉ nhận user context/token đã xác thực.

`video-service` quản lý metadata và upload flow. Service này không xử lý FFmpeg trực tiếp.

`media-worker` xử lý tác vụ nặng bất đồng bộ. Đây là nguồn incident quan trọng cho OOM, queue lag và retry storm.

`feed-social-service` phục vụ feed và tương tác xã hội cơ bản.

`live-service` quản lý live session và tích hợp MediaMTX.

## AIOps Service

`aiops-service` không sở hữu business data của product. Nó đọc evidence từ observability, Kubernetes, CI/CD, GitOps, security reports và runbooks.

## Boundary Quan Trọng

- Product service không gọi trực tiếp agent.
- AIOps service không tự deploy hoặc apply trực tiếp vào cluster.
- Remediation đi qua đề xuất hoặc GitOps MR.
- Evidence phải có source, timestamp, service, namespace và confidence.
