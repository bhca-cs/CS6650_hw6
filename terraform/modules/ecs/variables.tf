variable "service_name" {
  type        = string
  description = "Base name for ECS resources"
}

variable "image" {
  type        = string
  description = "ECR image URI (with tag)"
}

variable "container_port" {
  type        = number
  description = "Port your app listens on"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for FARGATE tasks"
}

variable "security_group_ids" {
  type        = list(string)
  description = "SGs for FARGATE tasks"
}

variable "execution_role_arn" {
  type        = string
  description = "ECS Task Execution Role ARN"
}

variable "task_role_arn" {
  type        = string
  description = "IAM Role ARN for app permissions"
}

variable "log_group_name" {
  type        = string
  description = "CloudWatch log group name"
}

variable "ecs_count" {
  type        = number
  default     = 2
  description = "Initial Fargate task count (auto scaling adjusts it afterwards)"
}

variable "region" {
  type        = string
  description = "AWS region (for awslogs driver)"
}

variable "cpu" {
  type        = string
  default     = "256"
  description = "vCPU units"
}

variable "memory" {
  type        = string
  default     = "512"
  description = "Memory (MiB)"
}

variable "target_group_arn" {
  type        = string
  description = "ALB target group the service registers its tasks into"
}

variable "min_capacity" {
  type        = number
  default     = 2
  description = "Fewest tasks auto scaling may run"
}

variable "max_capacity" {
  type        = number
  default     = 4
  description = "Most tasks auto scaling may run"
}

variable "cpu_target" {
  type        = number
  default     = 70
  description = "Average CPU % that target tracking aims for"
}

variable "scale_out_cooldown" {
  type        = number
  default     = 300
  description = "Seconds to wait after scaling out before scaling out again"
}

variable "scale_in_cooldown" {
  type        = number
  default     = 300
  description = "Seconds to wait after scaling in before scaling in again"
}
