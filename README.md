# AIOps Multi-Agent RAG on Kubernetes

Dự án xây dựng một nền tảng video/livestream dạng microservices chạy trên Kubernetes, đồng thời dùng chính hệ thống này làm môi trường thực nghiệm cho AIOps RCA bằng Evidence-aware Multi-Agent RAG.

Mục tiêu không chỉ là demo DevOps hay một app video đơn lẻ. Sản phẩm video/livestream là case study đủ thật để sinh sự cố vận hành, thu thập logs/metrics/events/deployment history, sau đó đánh giá khả năng phân tích nguyên nhân gốc bằng RAG và nhiều agent chuyên trách.

Roadmap HTML chi tiết được giữ ngoài repo source tại `../lo_trinh_trien_khai_aiops_multiagent_rag.html` để xem local và không push lên GitHub.

## Định Hướng Chính

Hệ thống được triển khai theo ba lớp:

1. Product Core
   - Identity/auth.
   - Video upload.
   - Media processing worker.
   - Feed/social cơ bản.
   - Livestream.
   - Admin dashboard.
   - Mobile app demo.

2. AIOps Core
   - Structured logs, metrics, health checks.
   - Incident context schema.
   - Data collector từ Kubernetes, Loki, Prometheus, Argo CD, GitLab, Harbor/Trivy và runbooks.
   - Qdrant vector database.
   - Log Agent, Metric Agent, Deployment Agent, Planner Agent.
   - RCA report có evidence, confidence score và recommended action.

3. Enterprise DevSecOps Layer
   - GitLab CI.
   - Container image build.
   - Security scan với Gitleaks/Trivy.
   - SBOM.
   - Harbor registry.
   - GitOps manifests.
   - Argo CD deployment.
   - Smoke test và rollback bằng GitOps.

## Kubernetes-First

Dự án sẽ dùng Kubernetes làm nền triển khai chính. Local development có thể dùng Docker Compose để chạy nhanh database, object storage, queue và service phụ, nhưng mục tiêu triển khai, demo và đánh giá sẽ dựa trên Kubernetes.

Platform baseline:

- k3s hoặc kubeadm.
- Nginx Ingress.
- cert-manager.
- Argo CD.
- Prometheus.
- Grafana.
- Loki.
- PostgreSQL.
- Redis.
- MinIO.
- Redpanda/Kafka.
- Qdrant.
- Harbor nếu đủ tài nguyên.

Thứ tự triển khai ưu tiên:

```text
App local skeleton
→ Docker/container baseline
→ Kubernetes baseline
→ Deploy app tối thiểu lên Kubernetes
→ Logging/metrics/events
→ CI build/push/deploy
→ DevSecOps scan/SBOM/Harbor
→ RAG/Multi-Agent RCA
→ Incident dataset và evaluation
```

## Kiến Trúc Dự Kiến

```text
[Flutter Mobile]        [Next.js Admin]
       |                    |
       +------ [Ingress / API Gateway] ------+
                         |
       +-----------------+-------------------+
       |                 |                   |
[identity-service] [video-service] [feed-social-service]
       |                 |                   |
 [PostgreSQL/Redis] [MinIO + Redpanda] [PostgreSQL/Redis]
                         |
                  [media-worker + FFmpeg]
                         |
                  [live-service + MediaMTX]

Developer/MR
   -> GitLab CI
   -> Security Scan / SBOM / Image Build
   -> Harbor Registry
   -> GitOps Repo MR
   -> Argo CD Sync
   -> Kubernetes

Prometheus + Loki + Kubernetes Events + Argo CD + GitLab + Harbor + Runbooks
   -> Data Collector
   -> Qdrant Vector DB
   -> Multi-Agent RAG
   -> RCA Report + Evidence + Safe Remediation Proposal
```

## Service Dự Kiến

```text
aiops-multi-agent-rag-k8s/
├── README.md
├── .gitlab-ci.yml
├── Makefile
├── docker-compose.yml
│
├── services/
│   ├── identity-service/
│   ├── video-service/
│   ├── feed-social-service/
│   ├── live-service/
│   ├── media-worker/
│   └── aiops-service/
│
├── apps/
│   ├── admin-web/
│   └── mobile-flutter/
│
├── packages/
│   ├── proto/
│   ├── contracts/
│   └── shared-docs/
│
├── deploy/
│   ├── docker-compose/
│   ├── k8s/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── dev/
│   │       └── demo/
│   ├── helm/
│   └── scripts/
│
├── infra/
│   ├── k3s/
│   ├── argocd/
│   ├── ingress/
│   ├── observability/
│   │   ├── prometheus/
│   │   ├── grafana/
│   │   └── loki/
│   ├── storage/
│   │   ├── postgres/
│   │   ├── redis/
│   │   └── minio/
│   └── registry/
│       └── harbor/
│
├── docs/
│   ├── architecture/
│   ├── api/
│   ├── runbooks/
│   ├── incidents/
│   ├── experiments/
│   └── thesis/
│
├── tests/
│   ├── e2e/
│   ├── smoke/
│   └── load/
│
└── tools/
    ├── incident-injector/
    ├── log-generator/
    └── rca-evaluator/
```

Repo companion cho GitOps:

```text
../aiops-gitops-manifests/
  argocd-apps/
  environments/
  platform/
```

## Tech Stack Đề Xuất

- Backend product services: Go.
- AIOps/RAG service: Python FastAPI.
- Admin dashboard: Next.js.
- Mobile app: Flutter.
- Database: PostgreSQL.
- Cache/session: Redis.
- Object storage: MinIO.
- Queue/event streaming: Redpanda/Kafka.
- Livestream: MediaMTX.
- Observability: Prometheus, Grafana, Loki.
- Vector database: Qdrant.
- Deployment: Kubernetes, Helm/Kustomize, Argo CD.
- CI/CD: GitLab CI, Harbor, Trivy, Gitleaks, SBOM.

## Product Flow MVP

Luồng sản phẩm đầu tiên cần chạy chắc:

```text
Login
→ Upload video
→ video-service lưu metadata và phát event
→ media-worker consume event
→ FFmpeg xử lý video/thumbnail
→ cập nhật trạng thái video
→ feed/admin hiển thị video ready
→ tạo incident có chủ đích
→ AIOps service gom evidence
→ xuất RCA report
```

## Incident Dataset Dự Kiến

Mỗi incident cần có script tái hiện, ground truth, evidence kỳ vọng, cleanup script và RCA output mẫu.

- INC-01: `video-service` CrashLoopBackOff do thiếu env/secret sau deploy.
- INC-02: `media-worker` OOMKilled khi xử lý video lớn.
- INC-03: Queue lag tăng do thiếu worker replica hoặc downstream chậm.
- INC-04: MinIO AccessDenied do sai credential hoặc bucket policy.
- INC-05: Feed latency cao do query chậm hoặc Redis unavailable.
- INC-06: Livestream latency cao do MediaMTX quá tải hoặc config protocol chưa phù hợp.
- INC-07: Canary/new release gây 5xx.
- INC-08: GitOps drift do sửa thủ công ngoài Git.

## Nguyên Tắc Thiết Kế AIOps

- Evidence trước, kết luận sau.
- Mọi RCA report phải chỉ ra nguồn evidence cụ thể.
- Không tự động `kubectl apply` trực tiếp vào cluster.
- Remediation nên tạo đề xuất hoặc GitOps MR để con người duyệt.
- Logs và context phải được redaction trước khi embedding hoặc gửi vào LLM.
- RCA scoring là evidence-weighted heuristic scoring, không trình bày như mô hình học máy nếu chưa có huấn luyện/kiểm chứng trọng số.

## Roadmap Ngắn Gọn

1. Chốt kiến trúc, service boundary, incident model.
2. Scaffold monorepo và backend skeleton.
3. Xây identity, video upload, media-worker.
4. Thêm admin dashboard và instrumentation.
5. Đưa app tối thiểu lên Kubernetes.
6. Cài observability và GitOps.
7. Hoàn thiện CI/CD, scan, registry, SBOM.
8. Xây data collector, Qdrant, Single-Agent RAG.
9. Xây Multi-Agent RAG và RCA scoring.
10. Tạo incident dataset, chạy evaluation, viết báo cáo.

## Tài Liệu Liên Quan

- [Cấu trúc repo](./docs/architecture/repo-structure.md)
- [Service boundaries](./docs/architecture/service-boundaries.md)
- Roadmap HTML local-only: `../lo_trinh_trien_khai_aiops_multiagent_rag.html`
