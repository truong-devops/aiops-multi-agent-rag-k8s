# Deploy

Thư mục này chứa cấu hình triển khai phục vụ development và template.

GitOps repo riêng `aiops-gitops-manifests` mới là nguồn thật để Argo CD sync. Repo source này chỉ giữ template, local compose và script hỗ trợ.

## Subdirectories

- `docker-compose`: local dependencies và local service composition.
- `k8s`: manifest/Kustomize template dùng khi phát triển.
- `helm`: chart template nếu service cần Helm.
- `scripts`: script hỗ trợ build, smoke test, render manifest.
