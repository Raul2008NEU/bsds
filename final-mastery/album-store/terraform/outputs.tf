output "base_url" {
  description = "Base URL of the application (ALB DNS)"
  value       = "http://${aws_lb.main.dns_name}"
}

output "alb_dns_name" {
  description = "Raw ALB DNS name (use for submit.sh)"
  value       = aws_lb.main.dns_name
}

output "db_endpoint" {
  description = "RDS PostgreSQL endpoint"
  value       = aws_db_instance.postgres.address
}

output "s3_bucket_name" {
  description = "S3 bucket name for photo storage"
  value       = aws_s3_bucket.photos.id
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.main.name
}
