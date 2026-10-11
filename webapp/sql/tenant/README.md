# スコアの重複行削除（Issue #11）

CSV受付時には全行を検証し、参加者ごとの最後の行だけを保存する。
参加者の存在確認はリクエスト内で再利用する。応答の `rows` は保存件数ではなく
CSVの入力行数で、同点順位に使う `row_num` は元CSVの行番号を維持する。

`30_compact_scores.sql` は、既存データについて大会・テナント・参加者ごとの
最大 `row_num` より古い行だけを削除する。繰り返し適用しても結果は同じ。
`/initialize` では原本からDBを復元し、`20_indexes.sql` の後に適用するため、
配布初期データの原本は変更しない。新規テナントには既存の重複行がないため不要。

既存DBへの手動適用が必要なら、アプリ・ベンチを停止してバックアップを取った後、
リポジトリルートで対象DBを指定して実行する（通常は次の `/initialize` で適用される）。

```sh
sqlite3 -bail webapp/tenant_db/1.db < webapp/sql/tenant/20_indexes.sql
sqlite3 -bail webapp/tenant_db/1.db < webapp/sql/tenant/30_compact_scores.sql
```

DBファイルの物理サイズを縮める `VACUUM` は実行しない。
実データでの初期化所要時間・30秒制限とスコア改善はサーバー上のベンチで確認する。
取り消す場合はコードを戻し、初期化で配布データから復元する。
