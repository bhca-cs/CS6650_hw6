# Application Load Balancer in front of the ECS tasks.
# Internet -> ALB (port 80) -> target group (port 8080, health check /health) -> tasks

# Security group for the ALB itself: accept HTTP from anywhere.
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow HTTP from the internet to the ALB"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "HTTP from anywhere"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound (to reach the tasks)"
  }
}

# The load balancer. An ALB needs subnets in at least 2 availability zones;
# the default VPC's subnets cover several.
resource "aws_lb" "this" {
  name               = "${var.service_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.subnet_ids
}

# Target group: the set of healthy tasks the ALB sends requests to.
# target_type "ip" is required for Fargate (each task has its own IP).
resource "aws_lb_target_group" "this" {
  name        = "${var.service_name}-tg"
  port        = var.container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id

  # How long a stopping task keeps finishing in-flight requests before it is
  # removed. The default is 300s; 30s makes scale-in and task stops faster.
  deregistration_delay = 30

  health_check {
    path                = var.health_check_path
    protocol            = "HTTP"
    matcher             = "200"
    interval            = 30 # seconds between checks
    timeout             = 5
    healthy_threshold   = 2 # 2 consecutive successes -> healthy
    unhealthy_threshold = 3 # 3 consecutive failures -> unhealthy
  }
}

# Listener: the ALB accepts HTTP on port 80 and forwards to the target group.
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}
