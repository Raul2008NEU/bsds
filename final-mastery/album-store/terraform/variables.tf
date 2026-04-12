variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-west-2"
}

variable "app_name" {
  description = "Name prefix used for all resources"
  type        = string
  default     = "album-store"
}

variable "db_password" {
  description = "Password for the RDS PostgreSQL instance"
  type        = string
  sensitive   = true
}

variable "image_uri" {
  description = "Full ECR image URI (e.g. 123456789.dkr.ecr.us-east-1.amazonaws.com/album-store:latest)"
  type        = string
}

variable "ecs_task_cpu" {
  description = "Fargate task CPU units"
  type        = number
  default     = 2048
}

variable "ecs_task_memory" {
  description = "Fargate task memory (MiB)"
  type        = number
  default     = 4096
}

variable "ecs_desired_count" {
  description = "Number of ECS tasks to run"
  type        = number
  default     = 2
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}
