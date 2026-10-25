variable "service_name" {
  type        = string
  description = "Base name for ALB resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC for the ALB and target group"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the ALB (at least 2 availability zones)"
}

variable "container_port" {
  type        = number
  description = "Port the tasks listen on"
}

variable "health_check_path" {
  type        = string
  default     = "/health"
  description = "Path the target group uses to check task health"
}
