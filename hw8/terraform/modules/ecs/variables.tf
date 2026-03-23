variable "project_name" {
  type    = string
  default = "hw8-store"
}

variable "app_port" {
  type    = number
  default = 8080
}

variable "desired_count" {
  type    = number
  default = 2
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "alb_security_group_id" {
  type = string
}

variable "ecs_security_group_id" {
  type = string
}

variable "execution_role_arn" {
  description = "IAM role for ECS agent (ECR pull, logs)"
  type        = string
}

variable "task_role_arn" {
  description = "IAM role for the running container (DynamoDB access, etc.)"
  type        = string
}

# Database connection info
variable "db_hostname" {
  type = string
}

variable "db_name" {
  type = string
}

variable "db_username" {
  type = string
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "dynamodb_table_name" {
  type = string
}