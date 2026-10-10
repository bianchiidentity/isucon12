# ISUCON12 サーバーの初期状態

性能改善前に、このサーバーで観測したファイルを保存した作業用リポジトリ。
取得日時と範囲は `baseline/capture.json` に記録。
競技当日の未変更AMIそのものを保証するものではない。
稼働中の `/home/isucon/webapp` や `/etc` は変更せず、サービス停止やDB初期化も実施していない。

## 保存した内容

| パス | 内容・保存理由 |
| --- | --- |
| `webapp/` | 全言語の参考実装、依存定義・ロックファイル、SQLスキーマ、初期化処理、公開鍵、アプリ仕様。改善差分と初期化互換性の確認用 |
| `public/` | 配信中の静的ファイル |
| `bench/`, `data/` | サーバーに存在するベンチマーカーとデータ生成器のソース |
| `baseline/config/etc/nginx/` | nginx本体・サイト・モジュール設定、公開TLS証明書、DHパラメータ |
| `baseline/config/etc/mysql/`, `baseline/config/etc/redis/` | DB設定。MySQLの管理用認証ファイル `debian.cnf` は対象外 |
| `baseline/config/etc/systemd/`, `baseline/config/lib/systemd/` | アプリ・認証サーバー・nginx・MySQL・Redisの起動設定 |
| `baseline/config/etc/sysctl*`, `baseline/config/etc/security/` | カーネル・プロセス制限のファイル設定 |
| `baseline/config/etc/hosts` | ホスト名と接続先の確認用。別サーバーにはそのまま適用しない |
| `baseline/symlinks.json` | nginx有効サイト・モジュール、サービス有効化などのリンク対応。設定ファイルは実体として保存 |
| `baseline/inventory/` | インストール済みパッケージ、サービス状態、リソース情報、実効MySQL・sysctl設定、除外バイナリのハッシュ |
| `baseline/data/` | `initial_data/*.db` 100個と `webapp/sql/admin/90_data.sql` の圧縮分割アーカイブとSHA-256 |
| `scripts/setup-github.sh` | ISUCONサーバー専用SSH鍵の設定スクリプト |

DBパスワード `isucon` など、公式参考実装の既定値はソース・サービス設定に含む。
SSH秘密鍵、AWS/GitHub認証情報、TLS秘密鍵、JWT秘密鍵、実行ログ、可変のテナントDB、macOS付随ファイルは保存しない。
除外一覧は `baseline/excluded.txt` に記録。
初期SQLは既存の配布データであり、現在稼働中のMySQLをダンプしたものではない。

## 初期データの復元

```bash
# 新規クローンのリポジトリ内へ展開
./scripts/restore-initial-data.sh

# または、検証用の新規ディレクトリへ展開
./scripts/restore-initial-data.sh /tmp/isucon12-data-check
```

分割ファイルと展開後の全101ファイルのSHA-256を検証する。
既存データを上書きせず、MySQLへの投入やサービス再起動は行わない。
展開される初期SQLiteは `initial_data/`、初期SQLは `webapp/sql/admin/90_data.sql`。
実行用 `webapp/tenant_db/` は、この初期SQLiteから別途作成する。
`webapp/sql/init.sh` はDBの内容を変更するため、復旧対象・アプリ停止手順を確認してから使用する。

## 運用と復旧の注意

- 稼働中のGoアプリは `/home/isucon/webapp/go/isuports`、systemdの作業場所も `/home/isucon/webapp/go`。このクローンを編集するだけでは稼働中アプリは変わらない。
- 保存した `/etc` 設定は比較・復旧用。サーバー固有のパスとホスト名を確認して必要なファイルだけ適用し、その後 `nginx -t`、systemdのdaemon-reloadなどを実施する。
- 初期状態の関連サービス `isuports`, `blackauth`, `nginx`, `mysql`, `redis-server` はすべて enabled / active。取得結果は `baseline/inventory/services.txt`。
- `blackauth` はこのサーバーにソースがなく、実行ファイルに秘密鍵が埋め込まれているため除外した。復旧では元サーバーのバイナリを安全な方法で引き継ぐか、公式ソースから再構築する。`/etc/nginx/tls/key.pem` とJWT署名鍵も別途安全に引き継ぐ。Gitに含めない。
- ベンチ用 `benchmarker*.json` はこのサーバーに存在しない。ベンチ実行環境のデータは公式リリース等から別途用意する。
- この保存はファイルと設定の基準点。ライブDB、OSイメージ、AWS構成やネットワーク設定を含む完全なサーバーバックアップではない。

## 過去のISUCONを踏まえた選定根拠

ISUCON12公式リポジトリの構成（webapp / bench / public / nginx / data）を基礎に、
このサーバーで動作する設定と初期データも保存した。
ISUCON12の解説では `/initialize` による外部から観測できる初期状態の復元が要求されるため、
初期化処理・スキーマ・初期データを一緒に保存している。
ISUCON11マニュアルのホスト名・TLS設定に関する説明も踏まえ、hostsとTLS設定への参照を保存した。

- [ISUCON12公式実装・初期データ](https://github.com/isucon/isucon12-qualify)
- [ISUCON12予選の解説と講評](https://isucon.net/archives/56850281.html)
- [ISUCON11予選当日マニュアル](https://github.com/isucon/isucon11-qualify/blob/main/docs/manual.md)

参考実装のライセンスは `LICENSE`（公式リポジトリより取得）。
`initial-state` タグを初期状態の比較基準として使用する。
