output "alb_url" {
  description = "Base URL for the shopping cart API"
  value       = "http://${module.ecs.alb_dns_name}"
}

output "ecr_repository_url" {
  value = module.ecs.ecr_repository_url
}

output "rds_endpoint" {
  value = module.rds.endpoint
}

output "dynamodb_table" {
  value = module.dynamodb.table_name
}