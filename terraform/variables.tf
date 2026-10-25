# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650HW6"
}

variable "container_port" {
  type    = number
  default = 8080
}

# Initial task count; auto scaling takes over after the first deploy.
variable "ecs_count" {
  type    = number
  default = 2
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

# ===================== AUTO SCALING (HW6 Part III) =====================
# Change these for the exploration experiments, e.g.
#   terraform apply -auto-approve -var="cpu_target=50"

variable "min_tasks" {
  type    = number
  default = 2
}

variable "max_tasks" {
  type    = number
  default = 4
}

variable "cpu_target" {
  type    = number
  default = 70
}

variable "scale_out_cooldown" {
  type    = number
  default = 300
}

variable "scale_in_cooldown" {
  type    = number
  default = 300
}
