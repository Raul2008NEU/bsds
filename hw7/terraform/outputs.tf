    # These values are printed after "terraform apply"
# You'll need them for testing

output "alb_dns" {
  description = "URL to send requests to"
  value       = aws_lb.main.dns_name
}

output "ecr_repository_url" {
  description = "Where to push Docker images"
  value       = aws_ecr_repository.order_service.repository_url
}

output "sns_topic_arn" {
  description = "SNS topic ARN for order events"
  value       = aws_sns_topic.order_events.arn
}

output "sqs_queue_url" {
  description = "SQS queue URL for order processing"
  value       = aws_sqs_queue.order_queue.url
}