#!/usr/bin/env bash
set -euo pipefail

BASE="http://localhost:8080"
ALBUM_ID="00000000-0000-0000-0000-000000000001"
PASS=0
FAIL=0

check() {
  local desc="$1"
  local expected="$2"
  local actual="$3"
  if echo "$actual" | grep -q "$expected"; then
    echo "  PASS  $desc"
    PASS=$((PASS + 1))
  else
    echo "  FAIL  $desc"
    echo "        expected: $expected"
    echo "        got:      $actual"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> Health check"
RESP=$(curl -sf "$BASE/health")
check "GET /health returns ok" '"ok"' "$RESP"

echo ""
echo "==> Album endpoints"

RESP=$(curl -sf -X PUT "$BASE/albums/$ALBUM_ID" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Album","description":"smoke test","owner":"tester"}')
check "PUT /albums/:id creates album" '"album_id"' "$RESP"

RESP=$(curl -sf "$BASE/albums/$ALBUM_ID")
check "GET /albums/:id returns album" '"Test Album"' "$RESP"

RESP=$(curl -sf "$BASE/albums")
check "GET /albums returns list" '"album_id"' "$RESP"

echo ""
echo "==> Photo endpoints"

# Create a small test image (1x1 red pixel PNG, base64 decoded)
TMPFILE=$(mktemp /tmp/test_photo_XXXXXX.png)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > "$TMPFILE"

RESP=$(curl -sf -X POST "$BASE/albums/$ALBUM_ID/photos" \
  -F "photo=@$TMPFILE;type=image/png")
check "POST /albums/:id/photos returns 202 with processing status" '"processing"' "$RESP"

PHOTO_ID=$(echo "$RESP" | grep -o '"photo_id":"[^"]*"' | cut -d'"' -f4)
rm -f "$TMPFILE"

if [ -n "$PHOTO_ID" ]; then
  echo "    photo_id: $PHOTO_ID"

  # Poll up to 10s for completion
  echo "    waiting for worker to complete upload..."
  for i in $(seq 1 10); do
    sleep 1
    STATUS_RESP=$(curl -sf "$BASE/albums/$ALBUM_ID/photos/$PHOTO_ID" || true)
    if echo "$STATUS_RESP" | grep -q '"completed"'; then
      check "GET /albums/:id/photos/:photoId status=completed" '"completed"' "$STATUS_RESP"
      break
    fi
    if [ "$i" -eq 10 ]; then
      check "GET /albums/:id/photos/:photoId status=completed (timed out)" '"completed"' "$STATUS_RESP"
    fi
  done

  RESP=$(curl -sf -o /dev/null -w "%{http_code}" \
    -X DELETE "$BASE/albums/$ALBUM_ID/photos/$PHOTO_ID")
  check "DELETE /albums/:id/photos/:photoId returns 204" "204" "$RESP"
else
  echo "  SKIP  photo sub-tests (could not parse photo_id)"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "==> Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
