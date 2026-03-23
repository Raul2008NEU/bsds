output "endpoint" {
  description = "RDS MySQL endpoint (host:port)"
  value       = aws_db_instance.mysql.endpoint
}

output "hostname" {
  description = "RDS hostname only (no port)"
  value       = aws_db_instance.mysql.address
}

output "port" {
  value = aws_db_instance.mysql.port
}

output "db_name" {
  value = aws_db_instance.mysql.db_name
}