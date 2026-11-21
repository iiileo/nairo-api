#!/usr/bin/env bash
# 从 gzip SQL 恢复。用法：
#   ./scripts/restore_mysql.sh ./backups/nairo_app_YYYYMMDD_HHMMSS.sql.gz
set -euo pipefail
FILE="${1:-}"
if [[ -z "$FILE" || ! -f "$FILE" ]]; then
  echo "usage: $0 <backup.sql.gz>" >&2
  exit 1
fi
HOST="${MYSQL_HOST:-127.0.0.1}"
PORT="${MYSQL_PORT:-3306}"
USER="${MYSQL_USER:-root}"
PASS="${MYSQL_PASSWORD:-root}"
DB="${MYSQL_DBNAME:-nairo_app}"
echo "restoring $FILE -> $DB@$HOST:$PORT"
gunzip -c "$FILE" | mysql -h"$HOST" -P"$PORT" -u"$USER" -p"$PASS" "$DB"
echo "ok"
