#!/bin/bash
# ============================================================
# DynamoDB Performance Test — 150 operations
# 50 create_cart + 50 add_items + 50 get_cart
# Output: results/dynamodb_test_results.json
# ============================================================

set -e

ALB_URL="${1:?Usage: ./dynamo_load_test.sh <ALB_URL>}"
OUTPUT_FILE="../results/dynamodb_test_results.json"
CART_IDS=()

echo "=== DynamoDB Performance Test ==="
echo "Target: $ALB_URL"
echo ""

# Initialize JSON array
echo "[" > "$OUTPUT_FILE"
FIRST=true

add_result() {
  local op="$1" rt="$2" success="$3" status="$4" ts="$5"
  if [ "$FIRST" = true ]; then
    FIRST=false
  else
    echo "," >> "$OUTPUT_FILE"
  fi
  cat >> "$OUTPUT_FILE" <<EOF
  {
    "operation": "$op",
    "response_time": $rt,
    "success": $success,
    "status_code": $status,
    "timestamp": "$ts"
  }
EOF
}

# --- Phase 1: Create 50 carts ---
echo "Phase 1/3: Creating 50 carts..."
for i in $(seq 1 50); do
  TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  START=$(python3 -c "import time; print(time.time())")

  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$ALB_URL/dynamo/shopping-carts" \
    -H "Content-Type: application/json" \
    -d "{\"customer_id\": \"customer-$i\"}")

  END=$(python3 -c "import time; print(time.time())")

  STATUS=$(echo "$RESPONSE" | tail -1)
  BODY=$(echo "$RESPONSE" | sed '$d')
  RT=$(python3 -c "print(round(($END - $START) * 1000, 2))")

  CART_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || echo "")
  if [ -n "$CART_ID" ]; then
    CART_IDS+=("$CART_ID")
  fi

  SUCCESS=$( [ "$STATUS" = "201" ] && echo "true" || echo "false" )
  add_result "create_cart" "$RT" "$SUCCESS" "$STATUS" "$TS"

  printf "\r  Created %d/50" "$i"
done
echo ""
echo "  Collected ${#CART_IDS[@]} cart IDs"

# --- Phase 2: Add items to 50 carts ---
echo "Phase 2/3: Adding items to 50 carts..."
for i in $(seq 0 49); do
  CART_ID="${CART_IDS[$i]}"
  TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  START=$(python3 -c "import time; print(time.time())")

  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$ALB_URL/dynamo/shopping-carts/$CART_ID/items" \
    -H "Content-Type: application/json" \
    -d "{\"product_id\": \"prod-$i\", \"name\": \"Product $i\", \"quantity\": $((RANDOM % 5 + 1)), \"price\": $((RANDOM % 100 + 10)).99}")

  END=$(python3 -c "import time; print(time.time())")

  STATUS=$(echo "$RESPONSE" | tail -1)
  RT=$(python3 -c "print(round(($END - $START) * 1000, 2))")

  SUCCESS=$( [ "$STATUS" = "201" ] && echo "true" || echo "false" )
  add_result "add_items" "$RT" "$SUCCESS" "$STATUS" "$TS"

  printf "\r  Added items %d/50" "$((i+1))"
done
echo ""

# --- Phase 3: Get 50 carts ---
echo "Phase 3/3: Retrieving 50 carts..."
for i in $(seq 0 49); do
  CART_ID="${CART_IDS[$i]}"
  TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  START=$(python3 -c "import time; print(time.time())")

  RESPONSE=$(curl -s -w "\n%{http_code}" "$ALB_URL/dynamo/shopping-carts/$CART_ID")

  END=$(python3 -c "import time; print(time.time())")

  STATUS=$(echo "$RESPONSE" | tail -1)
  RT=$(python3 -c "print(round(($END - $START) * 1000, 2))")

  SUCCESS=$( [ "$STATUS" = "200" ] && echo "true" || echo "false" )
  add_result "get_cart" "$RT" "$SUCCESS" "$STATUS" "$TS"

  printf "\r  Retrieved %d/50" "$((i+1))"
done
echo ""

# Close JSON array
echo "" >> "$OUTPUT_FILE"
echo "]" >> "$OUTPUT_FILE"

# --- Summary ---
echo ""
echo "=== Test Complete ==="
echo "Results saved to: $OUTPUT_FILE"
echo ""
echo "Quick stats:"
python3 -c "
import json
with open('$OUTPUT_FILE') as f:
    data = json.load(f)

total = len(data)
success = sum(1 for d in data if d['success'])
times = [d['response_time'] for d in data]
print(f'  Total ops:    {total}')
print(f'  Successes:    {success}/{total}')
print(f'  Avg RT:       {sum(times)/len(times):.2f} ms')
print(f'  Min RT:       {min(times):.2f} ms')
print(f'  Max RT:       {max(times):.2f} ms')

for op in ['create_cart', 'add_items', 'get_cart']:
    op_times = [d['response_time'] for d in data if d['operation'] == op]
    print(f'  {op:12s}:  avg {sum(op_times)/len(op_times):.2f} ms')
"