# pf-content-shortener

P08 URL 短縮（アイデア 09）のホットパスです。**学習用であり、本番短縮基盤の置き換えではありません。** 公開デモの作成先は許可ホストのみです。

リダイレクトは Next.js に載せません。レイテンシと悪用耐性がブログの SSG と別物だからです。

## できること（MVP）

- `POST /v1/links` で http(s) URL を短縮（開発ヘッダ `X-Dev-User-Sub`）
- `GET /:code` が **302**。クリック集計は応答後の非同期
- コードは予測困難（連番禁止）。カスタム slug は任意、数字のみは拒否
- `javascript:` / `data:` / 許可リスト外ホストは 400
- Redis に code→url を載せ、ミスしても Postgres で解決
- 生 IP は保存しない（ハッシュ計算のみ。集計テーブルは日次件数）

## 単体デモ

統合デモ（ブログ含む）は兄弟 `pf-content-infra` を使ってください。短縮だけなら:

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:8094/health | liveness |
| http://localhost:8094/ready | Postgres ping |

```powershell
curl -H "X-Dev-User-Sub: editor" -H "Content-Type: application/json" `
  -d '{"url":"http://localhost:3007/posts/why-redirect-is-not-nextjs"}' `
  http://localhost:8094/v1/links
```

返った `shortUrl` を開くと 302 します。ブログが無いと到着先は 接続失敗になり得ます。

## テスト

```powershell
go test ./...
```

メモリ実装。Redis / Postgres は Compose 用。integration タグはこのスライスに無い。

## 既知の制限

- P01 OIDC 未配線（`SHORTENER_DEV_AUTH=true`）
- レート制限・QR・パスワード付きリンク・k6 は未着手
- 日次グラフは件数テーブルまで。管理 UI はブログ側
- overlay E / K8s は未着手

設計: `project/portfolio-plan/content-platform/DESIGN.md`
