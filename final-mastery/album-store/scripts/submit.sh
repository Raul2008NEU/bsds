#!/usr/bin/env bash
set -euo pipefail

# Submit to ChaosArena
# Usage: ./scripts/submit.sh <alb-dns-name>
# Example: ./scripts/submit.sh my-alb-123456.us-east-1.elb.amazonaws.com

ALB_DNS="${1:-}"

if [ -z "$ALB_DNS" ]; then
  echo "Usage: $0 <alb-dns-name>"
  echo ""
  echo "Get the ALB DNS from Terraform:"
  echo "  cd terraform && terraform output base_url"
  exit 1
fi

BASE_URL="http://$ALB_DNS"

echo "==> Verifying service is reachable at $BASE_URL ..."
HEALTH=$(curl -sf --max-time 10 "$BASE_URL/health" || true)
if ! echo "$HEALTH" | grep -q '"ok"'; then
  echo "ERROR: /health check failed. Response: $HEALTH"
  echo "Make sure the ECS service is running and the ALB target group is healthy."
  exit 1
fi
echo "    health check passed"

echo ""
echo "==> Submitting to ChaosArena..."
echo "    BASE_URL=$BASE_URL"
echo ""
echo "    Replace the lines below with the actual ChaosArena submission command."
echo "    Example:"
echo "      curl -X POST https://chaosarena.example.com/submit \\"
echo "        -H 'Content-Type: application/json' \\"
echo "        -d '{\"url\": \"$BASE_URL\", \"team\": \"<your-team-id>\"}'"
echo ""
echo "==> Done."
