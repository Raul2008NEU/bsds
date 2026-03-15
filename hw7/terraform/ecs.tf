# =============================================
# ECR REPOSITORY
# =============================================

resource "aws_ecr_repository" "order_service" {
  name         = "order-service"
  force_delete = true
}

# =============================================
# ECS CLUSTER
# =============================================

resource "aws_ecs_cluster" "main" {
  name = "order-processing-cluster"
}

# =============================================
# NOTE: No IAM roles created here!
# AWS Academy Learner Lab doesn't allow iam:CreateRole.
# We use the pre-existing "LabRole" for everything.
# =============================================

# =============================================
# SECURITY GROUPS
# =============================================

resource "aws_security_group" "alb_sg" {
  name   = "alb-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "ecs_sg" {
  name   = "ecs-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb_sg.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# =============================================
# APPLICATION LOAD BALANCER
# =============================================

resource "aws_lb" "main" {
  name               = "order-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb_sg.id]
  subnets            = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

resource "aws_lb_target_group" "receiver" {
  name        = "receiver-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/health"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.receiver.arn
  }
}

# =============================================
# CLOUDWATCH LOG GROUP
# =============================================

resource "aws_cloudwatch_log_group" "ecs_logs" {
  name              = "/ecs/order-service"
  retention_in_days = 7
}

# =============================================
# ECS TASK DEFINITIONS — Using LabRole for both execution and task roles
# =============================================

resource "aws_ecs_task_definition" "receiver" {
  family                   = "order-receiver"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = var.lab_role_arn
  task_role_arn            = var.lab_role_arn

  container_definitions = jsonencode([{
    name  = "order-receiver"
    image = "${aws_ecr_repository.order_service.repository_url}:latest"
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "SERVICE_ROLE",  value = "receiver" },
      { name = "SNS_TOPIC_ARN", value = aws_sns_topic.order_events.arn },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.order_queue.url },
      { name = "PORT",          value = "8080" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/order-service"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "receiver"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "processor" {
  family                   = "order-processor"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = var.lab_role_arn
  task_role_arn            = var.lab_role_arn

  container_definitions = jsonencode([{
    name  = "order-processor"
    image = "${aws_ecr_repository.order_service.repository_url}:latest"
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "SERVICE_ROLE",  value = "processor" },
      { name = "SNS_TOPIC_ARN", value = aws_sns_topic.order_events.arn },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.order_queue.url },
      { name = "NUM_WORKERS",   value = tostring(var.num_workers) },
      { name = "PORT",          value = "8080" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = "/ecs/order-service"
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "processor"
      }
    }
  }])
}

# =============================================
# ECS SERVICES
# =============================================

resource "aws_ecs_service" "receiver" {
  name            = "order-receiver"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.receiver.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.private_1.id, aws_subnet.private_2.id]
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.receiver.arn
    container_name   = "order-receiver"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]
}

resource "aws_ecs_service" "processor" {
  name            = "order-processor"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.processor.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.private_1.id, aws_subnet.private_2.id]
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = false
  }
}