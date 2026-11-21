#!/usr/bin/env bash
# 将当前工作树以「无私有历史」方式强制同步到 public remote（iiileo/nairo-api）。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REMOTE="${PUBLIC_REMOTE:-public}"
BRANCH="${PUBLIC_BRANCH:-main}"
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

rsync -a \
  --exclude '.git/' \
  --exclude '.env' \
  --exclude '.env.local' \
  --exclude 'node_modules/' \
  --exclude '**/node_modules/' \
  --exclude 'dist/' \
  --exclude '**/dist/' \
  --exclude 'bin/' \
  --exclude '.DS_Store' \
  --exclude 'backups/' \
  --exclude '*.sql.gz' \
  --exclude 'internal/webembed/data/index/***' \
  --exclude 'internal/webembed/data/console/***' \
  --exclude 'internal/webembed/data/admin/***' \
  "$ROOT/" "$TMP/"

mkdir -p "$TMP/internal/webembed/data/"{index,console,admin}
for d in index console admin; do
  touch "$TMP/internal/webembed/data/$d/.gitkeep"
done

cd "$TMP"
if [ -f .env ]; then
  echo "refusing to publish .env" >&2
  exit 1
fi
git init -q -b "$BRANCH"
git add -A
MSG="${1:-Sync public tree from private development repo.}"
git -c user.name="${GIT_AUTHOR_NAME:-iiileo}" \
    -c user.email="${GIT_AUTHOR_EMAIL:-iiileo@users.noreply.github.com}" \
    commit -q -m "$MSG"

URL="$(git -C "$ROOT" remote get-url "$REMOTE")"
git remote add origin "$URL"
git push -f origin "HEAD:$BRANCH"
echo "Pushed clean tree to $URL ($BRANCH), commits=$(git rev-list --count HEAD)"
