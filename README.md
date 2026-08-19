# pf-content-shortener

学習用の URL 短縮です。リダイレクトはブログ（Next.js）に載せません。レイテンシと悪用耐性が静的配信と別物だからです。**本番短縮サービスの置き換えではありません。**

公開デモで作れる短縮先は、許可したホストだけです。

## できること

- `POST /v1/links` で https の URL を短縮する（開発ヘッダ `X-Dev-User-Sub`）
- `GET /:code` が 302。クリック集計は応答のあと非同期
- コードは連番ではありません。数字だけのカスタム slug は拒否します
- `javascript:` や許可リスト外ホストは 400
- Redis に載せて、欠けても Postgres で解決します
- 生 IP は保存しません

## 起動

ブログ込みは [pf-content-infra](https://github.com/maeplego/pf-content-infra) が簡単です。短縮だけなら:

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:8094/health | ヘルス |
| http://localhost:8094/ready | Postgres |

```powershell
curl -H "X-Dev-User-Sub: editor" -H "Content-Type: application/json" `
  -d '{"url":"http://localhost:3007/posts/why-redirect-is-not-nextjs"}' `
  http://localhost:8094/v1/links
```

返った `shortUrl` を開くと 302 します。到着先のブログが無いと接続失敗になります。

## テスト

```powershell
go test ./...
```

レート制限、QR、パスワード付きリンクはありません。日次件数テーブルはあり、グラフ UI はブログ側です。

設計の詳細は [portfolio-plan](https://github.com/maeplego/portfolio-plan) の `portfolio-plan/content-platform/docs/` です。
