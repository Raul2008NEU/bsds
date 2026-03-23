variable "project_name" {
  type    = string
  default = "hw8-store"
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for the DB subnet group"
  type        = list(string)
}

variable "rds_security_group_id" {
  description = "Security group ID allowing ECS access on 3306"
  type        = string
}

variable "db_name" {
  description = "Name of the initial database"
  type        = string
  default     = "shopdb"
}

variable "db_username" {
  description = "Master username"
  type        = string
  default     = "admin"
}

variable "db_password" {
  description = "Master password"
  type        = string
  sensitive   = true
}