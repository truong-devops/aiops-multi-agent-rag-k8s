# aiops-service

AIOps service cho RCA bằng Evidence-aware Multi-Agent RAG.

Runtime cố định: Python `3.12.13`, Docker image `python:3.12.13-slim-bookworm`.

Dependency trực tiếp được pin trong `pyproject.toml`; khi cài dependency mới phải cập nhật [Dependency Versioning](../../docs/development/dependency-versioning.md).

## Trách Nhiệm

- Nhận incident context.
- Thu thập evidence từ Kubernetes, Loki, Prometheus, Argo CD, GitLab, Harbor/Trivy và runbooks.
- Redaction dữ liệu nhạy cảm.
- Chunking, embedding và lưu Qdrant.
- Điều phối Planner, Log, Metric và Deployment Agent.
- Xuất RCA report có evidence, confidence score và recommended action.

## Convention Dự Kiến

```text
aiops-service/
├── app/
│   ├── api/
│   ├── core/
│   ├── collectors/
│   ├── rag/
│   ├── agents/
│   ├── scoring/
│   ├── redaction/
│   └── schemas/
├── tests/
├── .python-version
├── Dockerfile
├── pyproject.toml
└── README.md
```

## Dependencies Dự Kiến

- MongoDB for incidents, evidence items, RCA reports, agent runs and runbook chunks.
- Redis for analysis locks, collector cache and progress cache.
- Kubernetes API.
- Loki.
- Prometheus.
- Argo CD API.
- GitLab API.
- Harbor/Trivy reports.
- Qdrant.
- LLM/embedding provider.
