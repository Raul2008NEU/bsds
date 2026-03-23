# ============================================================
# DynamoDB Module — Single-table design for Shopping Carts
# ============================================================
#
# Design Decision: Single table with cart_id as partition key.
# Items are stored as a JSON list attribute embedded in the cart item.
# This avoids the need for Query operations and keeps retrieval to
# a single GetItem call — ideal for the simple access patterns here.
#
# Why NOT a sort key?
#   - We never need to query a range within a cart (e.g., "items 1-10")
#   - All access is by cart_id → GetItem is O(1) and cheaper than Query
#   - Items embedded as a list attribute keeps things simple
#
# GSI on customer_id enables "get all carts for customer" if needed.

resource "aws_dynamodb_table" "carts" {
  name         = "${var.project_name}-carts"
  billing_mode = "PAY_PER_REQUEST" # No capacity planning needed

  hash_key = "cart_id"

  attribute {
    name = "cart_id"
    type = "S"
  }

  attribute {
    name = "customer_id"
    type = "S"
  }

  global_secondary_index {
    name            = "customer-index"
    hash_key        = "customer_id"
    projection_type = "ALL"
  }

  tags = {
    Name = "${var.project_name}-carts"
  }
}