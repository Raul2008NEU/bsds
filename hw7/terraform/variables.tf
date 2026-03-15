terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  description = "AWS region to deploy into"
  default     = "us-east-1"
}

variable "num_workers" {
  description = "Number of worker goroutines in the processor task"
  default     = 1
}

# AWS Academy Learner Lab provides this pre-made role
# Run: aws iam get-role --role-name LabRole --query 'Role.Arn' --output text
variable "lab_role_arn" {
  description = "ARN of the pre-existing LabRole from AWS Academy"
  default     = "arn:aws:iam::270387470048:role/LabRole"
}