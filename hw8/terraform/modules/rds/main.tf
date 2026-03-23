# ============================================================
# RDS Module — MySQL 8.0 on db.t3.micro (Free Tier)
# ============================================================

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnet"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "${var.project_name}-db-subnet-group"
  }
}

resource "aws_db_instance" "mysql" {
  identifier = "${var.project_name}-mysql"

  # Engine
  engine         = "mysql"
  engine_version = "8.0"

  # Instance size (free tier)
  instance_class = "db.t3.micro"

  # Storage
  allocated_storage     = 20
  max_allocated_storage = 50
  storage_type          = "gp2"

  # Credentials
  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  # Networking
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [var.rds_security_group_id]
  publicly_accessible    = false

  # Assignment-specific settings
  skip_final_snapshot    = true
  deletion_protection    = false

  # Backups (minimal for assignment)
  backup_retention_period = 0

  # Monitoring
  monitoring_interval = 0

  tags = {
    Name = "${var.project_name}-mysql"
  }
}