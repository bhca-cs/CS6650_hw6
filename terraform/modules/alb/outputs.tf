output "target_group_arn" {
  description = "Target group the ECS service registers its tasks into"
  value       = aws_lb_target_group.this.arn
}

output "alb_dns_name" {
  description = "Public DNS name of the load balancer"
  value       = aws_lb.this.dns_name
}

# The "arn_suffix" values are what CloudWatch uses to identify the ALB and
# target group in its metrics (request count, response time, healthy hosts).
output "alb_arn_suffix" {
  value = aws_lb.this.arn_suffix
}

output "target_group_arn_suffix" {
  value = aws_lb_target_group.this.arn_suffix
}
