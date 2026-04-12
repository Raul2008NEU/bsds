#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

DATABASE_URL="postgres://album:album@localhost:5432/albumdb"
BUCKET="album-photos"

echo "==> Starting services..."
docker compose -f "$ROOT_DIR/docker-compose.yml" up -d --build

echo "==> Waiting for postgres..."
until docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T postgres \
  pg_isready -U album -d albumdb &>/dev/null; do
  sleep 1
done
echo "    postgres ready"

echo "==> Waiting for localstack..."
until curl -sf http://localhost:4566/_localstack/health | grep -q '"s3"'; do
  sleep 1
done
echo "    localstack ready"

echo "==> Running migrations..."
docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T postgres \
  psql -U album -d albumdb < "$ROOT_DIR/migrations/001_create_albums.sql"
docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T postgres \
  psql -U album -d albumdb < "$ROOT_DIR/migrations/002_create_photos.sql"
echo "    migrations applied"

echo "==> Creating S3 bucket..."
aws --endpoint-url=http://localhost:4566 \
    --region us-east-1 \
    s3 mb "s3://$BUCKET" 2>/dev/null || echo "    bucket already exists"

echo ""
echo "==> All set! App running at http://localhost:8080"
echo "    Run ./scripts/test_local.sh to smoke-test the endpoints."
