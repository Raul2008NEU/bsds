# =============================================
# SNS TOPIC — The "broadcast" channel
# When an order comes in, we publish it here.
# SNS fans it out to all subscribers (SQS, Lambda, etc.)
# =============================================

resource "aws_sns_topic" "order_events" {
  name = "order-processing-events"
}

# =============================================
# SQS QUEUE — The "buffer"
# Messages sit here until a worker picks them up.
# This is what lets us decouple "accepting orders" from "processing orders"
# =============================================

resource "aws_sqs_queue" "order_queue" {
  name = "order-processing-queue"

  # If a worker grabs a message but doesn't delete it within 30s,
  # the message becomes visible again for another worker to try
  visibility_timeout_seconds = 30

  # Messages stay in the queue for up to 4 days if not processed
  message_retention_seconds = 345600    # 4 days

  # Long polling: wait up to 20s for messages instead of returning empty immediately
  # This reduces the number of empty API calls (saves money + reduces latency)
  receive_wait_time_seconds = 20
}

# =============================================
# SNS → SQS SUBSCRIPTION
# "Hey SNS, whenever a message is published, send a copy to this SQS queue"
# =============================================

resource "aws_sns_topic_subscription" "sqs_sub" {
  topic_arn = aws_sns_topic.order_events.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.order_queue.arn
}

# Permission: Allow SNS to send messages into SQS
resource "aws_sqs_queue_policy" "allow_sns" {
  queue_url = aws_sqs_queue.order_queue.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowSNSPublish"
        Effect    = "Allow"
        Principal = { Service = "sns.amazonaws.com" }
        Action    = "sqs:SendMessage"
        Resource  = aws_sqs_queue.order_queue.arn
        Condition = {
          ArnEquals = {
            "aws:SourceArn" = aws_sns_topic.order_events.arn
          }
        }
      }
    ]
  })
}