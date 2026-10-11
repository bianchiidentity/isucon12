# 既存MySQLへのマイグレーション

## 002: 閲覧履歴を初回閲覧へ集約（Issue #13）

**Goの新コードを起動する前に、アプリとベンチを停止して一度だけ適用する。**
UNIQUE制約のない旧テーブルではUPSERTが重複行を防げないため、コードだけの反映は不可。
新規DBを新しい `admin/10_schema.sql` で作った場合は適用不要。

```sh
cd ~/isucon12
mysql -h 127.0.0.1 -u isucon -p isuports < webapp/sql/migrations/002_first_visit_history.sql
```

テナント・大会・参加者ごとの `MIN(created_at)` を新テーブルへコピーし、
複合主キーで1行に制約する。コピー完了後、RENAME TABLEで一度に切り替える。
旧テーブルは `visit_history_before_first_visit` として保持するため、一時的に
新旧両方の容量が必要。`mysql --force` は使わない。途中で失敗した場合や適用済みの場合は
再実行せず、旧・新・作業用テーブルの状態を確認する。

書き込みはUPSERTと `LEAST` で時刻の最小値を保つ。後から古い時刻の要求が到着しても
反映され、単に到着が早かった要求を採用する動作にはならない。
後の閲覧では `updated_at` も更新せず、最小値を保持する（請求では使用しない）。
請求は1人1行を読むため、毎回のGROUP BYは不要になる。

`/initialize` では再集約しない。従来と同じ境界時刻 `1654041600` 以降の行を削除する。
実行中の閲覧時刻がこの境界以降であれば、初期データの最小時刻は変わらず、
実行中だけに発生した閲覧者は削除される。削除用の時刻インデックスも追加する。
SQLite初期化は `47afc3e` のまま維持する。

移行前のコードへ戻すときは、アプリ・ベンチ停止中に次の切り替えを行い、
旧コードを起動する。バックアップへの復元なので、移行後の閲覧履歴は引き継がない。

```sql
RENAME TABLE visit_history TO visit_history_after_first_visit,
             visit_history_before_first_visit TO visit_history;
```

実MySQL用の統合テストは `webapp/go/visit_mysql_test.go`。
`ISUCON_TEST_MYSQL_DSN` を設定すると独立した一時DBを作成・削除して、移行・並行UPSERT・
請求の終了時刻境界・初期化の繰り返しを確認する。指定アカウントにはDB作成・削除権限が必要。
DSN未設定時はスキップする。今回の実装ではユーザーの指示により追加検証は実行していない。

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

## 003: 請求結果の保存と閲覧履歴の更新番号（Issue #16）

新しいGoアプリの起動前に、`003_billing_reports.sql`を一度だけ適用する。
履歴原本の書き換えはなく、追加するのは大会別の更新番号・請求人数の2テーブル。
新規DBは`admin/10_schema.sql`に含むため追加適用不要。

```sh
mysql -h 127.0.0.1 -u isucon -p isuports < webapp/sql/migrations/003_billing_reports.sql
```

閲覧履歴が追加・早い時刻へ更新されたとき、同じMySQLトランザクションで更新番号を進める。
請求結果は計算開始前の番号と終了時刻を保存し、現在の番号と一致する場合だけ再利用する。
同秒・遅着・計算中の閲覧でも不一致になれば再計算する。更新のない再閲覧は無効化しない。
終了後の新規閲覧でも安全側に再計算するため、確定済み大会でも必ず一回しか計算しないという設計ではない。
TTLや終了後の固定待ち時間には依存しない。

CSV登録・終了・請求再計算はテナントflockで排他し、保存値が有効ならスコア取得・flockを省く。
CSVはロック取得後にも終了を確認し、書き込み前に保存値を削除する。失敗時は再計算に戻る。
既存大会は初回請求で遅延作成し、再起動後も利用する。初期化は保存値・番号を消去する。
元の集計と同じ人数から金額を生成する。保存値にタイトルは持たず、大会の現在値を使う。

取り消す場合は旧アプリと旧init.sqlへ戻す。追加テーブルは残しても旧版には影響しない。
旧版の閲覧は番号を進めないため、再度新版に戻す前にbilling_reportsを消去すること。
実MySQLテストは独立した使い捨てDBで実行し、アプリ用アカウントの権限は拡張しない。
