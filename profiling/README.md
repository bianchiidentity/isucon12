# pprotein の計測環境

公式リリースの pprotein v1.2.4 / alp v1.0.22 / slp v0.2.3 と Graphviz を導入。
実行ファイルは `/home/isucon/tools/pprotein/bin`、収集データ・設定は
`/home/isucon/tools/pprotein/runtime/data` に保存する。収集データはGitに入れない。

## 管理画面

手元の端末からSSHトンネルを作成する（SERVER_IPは実際のサーバーに置き換える）。

```bash
ssh -N -L 9000:127.0.0.1:9000 isucon@SERVER_IP
```

ブラウザーで `http://localhost:9000` を開く。
pproteinの9000番とagentの19000番はsystemdの `IPAddressDeny=any` /
`IPAddressAllow=localhost` でローカル接続だけに制限する。
GoアプリのCPUプロファイルは `127.0.0.1:6060/debug/pprof/profile`。
agentのCPUプロファイルではなく、アプリ本体の6060番を収集先にしている。
fgprofは未導入。SQLiteは別の `sqlite` 集計として同じグループに表示する。

## ベンチと同時に収集

`targets.json` に4種類とも60秒の収集を設定した。
ベンチの初期化・整合性チェックが終わり、「負荷走行を開始します」が出たタイミングで
画面の収集操作、または次のAPIを実行する。

```bash
curl --fail http://127.0.0.1:9000/api/group/collect
```

収集対象は、GoアプリCPU / nginx LTSVアクセスログ / MySQLスローログ / SQLite SQLログ。
pproteinは収集リクエストを受け付けた後、バックグラウンドで収集・集計する。
ログ集計はagentが収集開始後に追加された行だけを取得する。
ベンチ終了後に収集を始めても、そのベンチのログは取得できない。

nginxは従来のアクセスログに加えて `/var/log/nginx/pprotein-access.log` を記録する。
MySQLは `/var/log/mysql/pprotein-slow.log` に `long_query_time=0` で全クエリを記録する。
計測負荷があるため、計測なしの初回スコア1854との比較にはその差を考慮する。
ベンチマーカーも同じサーバーで実行している。

## 適用済みの変更

- `nginx-pprotein.conf` → `/etc/nginx/conf.d/pprotein.conf`
- `nginx-isuports.conf` → `/etc/nginx/sites-available/isuports.conf`
- `mysql-pprotein.cnf` → `/etc/mysql/mysql.conf.d/zz-pprotein.cnf`
- `pprotein.service`, `pprotein-agent.service` → `/etc/systemd/system/`。両方enabled / active
- `alp.yml`, `slp.yml`, `targets.json` → pproteinの設定APIから適用済み
- `webapp/go/cmd/isuports/main.go` → 稼働ソースにも反映し、Go 1.18.4の既存Dockerイメージでビルド・配置済み

Goアプリのバイナリ・main.goとnginxの変更前サイト設定は
`/home/isucon/tools/pprotein/backup/` に保存した。
初期状態のGitタグ `initial-state` も維持している。
設定適用時にMySQLとGoアプリを再起動した。ベンチ実行時は通常どおり初期化される。

```bash
systemctl status pprotein pprotein-agent isuports --no-pager
sudo journalctl -u pprotein -u pprotein-agent -n 50 --no-pager
```

計測を止める際はpproteinサービスを止めるだけではMySQLの全クエリ記録は止まらない。
MySQLの追加設定も無効化してMySQLとアプリを再起動し、必要ならnginxの追加ログ設定も戻す。
変更前のMySQLはslow_query_log=OFF / long_query_time=10。

## 検証

3秒の収集窓で読み取りSQLとHTTPリクエストを発行し、3種類とも `Status=ok` / `Ready` を確認した。
CPUプロファイルの画面、alpとslpの集計データの取得も成功。
nginxの設定検査、全関連サービスの起動、ローカル接続制限を確認済み。

公式ソース: https://github.com/kaz/pprotein

## SQLiteの計測

`ISUCON_SQLITE_PROFILE_FILE` を指定し、Goアプリを `-tags sqlite_trace` でビルドする。
SQLiteの `SQLITE_TRACE_PROFILE` でSELECTの結果読み取り・終了までの実行時間を記録し、
slpが読み取れるMySQL互換形式で `/home/isucon/tools/pprotein/runtime/sqlite-slow.log` に出力する。
SQLのパラメータ値は展開しない。出力は排他制御し、書き込み失敗はアプリのSQL結果を変更せずログに出す。

SQLiteの報告時間はミリ秒精度で、1ms未満は0になる。SQL準備やアプリ側のflock待ちは含まれない。
Rows_sentは読み取った行数。Lock_timeとRows_examinedは取得していないため互換形式上0とし、分析には使わない。
全SQLの記録には負荷があるので、変更前後は同じ計測条件で比較する。

`pprotein-sqlite-agent.service` を `/etc/systemd/system/` に設置し、
`isuports-sqlite-profile.conf` を `/etc/systemd/system/isuports.service.d/` に設置する。
SQLite専用agentは19001番で、他のagentと同じくlocalhost接続に制限する。
`targets.json` のSQLite収集先を含めて設定APIへ適用する。

毎回のベンチと収集は次で実行する。負荷開始時に4種類を収集し、同一グループのReadyまで確認する。

```bash
python3 /home/isucon/isucon12/scripts/bench-with-pprotein.py
```

通常のビルドでは追加計測なしで動作する。計測用環境変数を残してタグなしビルドすると、
起動時に明示的なエラーにして計測漏れを防ぐ。既存のJSONトレーサーとは別の計測機能。
