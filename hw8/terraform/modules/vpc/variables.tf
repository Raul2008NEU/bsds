variable "project_name" {
  description = "Project name for resource tagging"
  type        = string
  default     = "hw8-store"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "app_port" {
  description = "Port the Go application listens on"
  type        = number
  default     = 8080
}