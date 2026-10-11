#!/bin/sh

set -ex
cd `dirname $0`

ISUCON_DB_HOST=${ISUCON_DB_HOST:-127.0.0.1}
ISUCON_DB_PORT=${ISUCON_DB_PORT:-3306}
ISUCON_DB_USER=${ISUCON_DB_USER:-isucon}
ISUCON_DB_PASSWORD=${ISUCON_DB_PASSWORD:-isucon}
ISUCON_DB_NAME=${ISUCON_DB_NAME:-isuports}

# MySQLを初期化
mysql -u"$ISUCON_DB_USER" \
		-p"$ISUCON_DB_PASSWORD" \
		--host "$ISUCON_DB_HOST" \
		--port "$ISUCON_DB_PORT" \
		"$ISUCON_DB_NAME" < init.sql

# SQLiteのデータベースを初期化
rm -f ../tenant_db/*.db
cp -r ../../initial_data/*.db ../tenant_db/

# 原本は変更せず、重複行を削除してから残った行に索引を作る。
for db in ../tenant_db/*.db; do
	sqlite3 -bail "$db" < tenant/30_compact_scores.sql
	sqlite3 -bail "$db" < tenant/20_indexes.sql
done
