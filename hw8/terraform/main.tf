# ============================================================
# Root — HW8 Online Store with MySQL + DynamoDB
# ============================================================

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

# --- Lookup the pre-existing LabRole (AWS Academy) ---
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# --- VPC ---
module "vpc" {
  source       = "./modules/vpc"
  project_name = var.project_name
  vpc_cidr     = var.vpc_cidr
  app_port     = var.app_port
}

# --- RDS MySQL ---
module "rds" {
  source                = "./modules/rds"
  project_name          = var.project_name
  private_subnet_ids    = module.vpc.private_subnet_ids
  rds_security_group_id = module.vpc.rds_security_group_id
  db_name               = var.db_name
  db_username           = var.db_username
  db_password           = var.db_password
}

# --- DynamoDB ---
module "dynamodb" {
  source       = "./modules/dynamodb"
  project_name = var.project_name
}

# --- ECS (Fargate + ALB) ---
module "ecs" {
  source = "./modules/ecs"

  project_name          = var.project_name
  app_port              = var.app_port
  desired_count         = var.desired_count
  vpc_id                = module.vpc.vpc_id
  public_subnet_ids     = module.vpc.public_subnet_ids
  private_subnet_ids    = module.vpc.private_subnet_ids
  alb_security_group_id = module.vpc.alb_security_group_id
  ecs_security_group_id = module.vpc.ecs_security_group_id
  execution_role_arn    = data.aws_iam_role.lab_role.arn
  task_role_arn         = data.aws_iam_role.lab_role.arn
  db_hostname           = module.rds.hostname
  db_name               = module.rds.db_name
  db_username           = var.db_username
  db_password           = var.db_password
  dynamodb_table_name   = module.dynamodb.table_name
}