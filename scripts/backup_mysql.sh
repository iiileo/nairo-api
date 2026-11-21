#!/usr/bin/env bash
# MySQL 逻辑备份。用法：
#   MYSQL_HOST=127.0.0.1 MYSQL_USER=root MYSQL_PASSWORD=root MYSQL_DBNAME=nairo_app ./scripts/backup_mysql.sh
set -euo pipefail
HOST="${MYSQL_HOST:-127.0.0.1}"
PORT="${MYSQL_PORT:-3306}"
USER="${MYSQL_USER:-root}"
PASS="${MYSQL_PASSWORD:-root}"
DB="${MYSQL_DBNAME:-nairo_app}"
OUT_DIR="${BACKUP_DIR:-./backups}"
mkdir -p "$OUT_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
FILE="$OUT_DIR/${DB}_${STAMP}.sql.gz"
echo "backing up $DB@$HOST:$PORT -> $FILE"
mysqldump -h"$HOST" -P"$PORT" -u"$USER" -p"$PASS" \
  --single-transaction --routines --triggers --default-character-set=utf8mb4 \
  "$DB" | gzip -c >"$FILE"
ls -lh "$FILE"
echo "ok"
