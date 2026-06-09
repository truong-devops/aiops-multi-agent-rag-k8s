# aiops-service

AIOps service cho RCA bằng Evidence-aware Multi-Agent RAG.

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
├── Dockerfile
├── pyproject.toml
└── README.md
```

## Dependencies Dự Kiến

- Kubernetes API.
- Loki.
- Prometheus.
- Argo CD API.
- GitLab API.
- Harbor/Trivy reports.
- Qdrant.
- LLM/embedding provider.
