# 既存MySQLへのマイグレーション

## 001: 閲覧履歴の請求集約用インデックス（Issue #3）

既存DBには、コードをpullした後、ベンチ開始前に一度だけ適用する。
`admin/10_schema.sql` はテーブルを削除するため、既存DBへの適用には使用しない。

リポジトリルートから実行する（接続先・ユーザー・DB名は環境に合わせる）。

```sh
mysql -h 127.0.0.1 -u isucon -p isuports < webapp/sql/migrations/001_visit_history_billing.sql
```

`visit_history_billing` がすでにある場合は再実行しない。新規DBでは
`admin/10_schema.sql` で作成されるため、このマイグレーションは不要。
既存の `tenant_id_idx` は維持する。`/initialize` は履歴のDELETEのみなので、
追加したインデックスは初期化後も残る。

適用前後に次のSQLで定義と実行計画を確認する。`?` は実在するテナントID・大会IDに置き換える。

```sql
SHOW INDEX FROM visit_history;
EXPLAIN SELECT player_id, MIN(created_at) AS min_created_at
FROM visit_history
WHERE tenant_id = ? AND competition_id = ?
GROUP BY player_id;
```

請求結果の一致、集約SQLの走査行数・時間、履歴INSERTの時間を比較する。
索引は書込時にも更新されるため、スコア改善はベンチで判断する。
`ALGORITHM=INPLACE, LOCK=NONE` を明示し、対応しない環境で別方式へ自動的に切り替えない。
オンラインDDLでも開始・終了時のメタデータロック待ちは発生し得るため、ベンチ中には適用しない。

取り消す場合（スキーマファイルの変更も戻す）：

```sql
ALTER TABLE visit_history DROP INDEX visit_history_billing,
  ALGORITHM=INPLACE, LOCK=NONE;
```
