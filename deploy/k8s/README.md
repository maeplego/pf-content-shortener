# P08 content-shortener Kubernetes manifests

短縮 + クリック計測（Go）。overlay smoke は `SHORTENER_DEV_AUTH` + `X-Dev-User-Sub`。単体 apply ではなく `pf-cloud-k8s` overlay `e-content` から参照する。

Ingress（`pf-cloud-k8s`）:

| ホスト | Service | 用途 |
| --- | --- | --- |
| `shortener.localhost` | shortener:8094 | `POST /v1/links` と `GET /:code` |

許可ホストは `blog.localhost`（オープンリダイレクト対策）。Redis は platform 共有。

```powershell
cd ..\..\pf-cloud-k8s
.\scripts\cluster-smoke-e-content.ps1
```
