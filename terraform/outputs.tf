output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

# Use this as the Locust host: http://<alb_dns_name>
output "alb_dns_name" {
  description = "Public DNS name of the load balancer"
  value       = module.alb.alb_dns_name
}

# Identifiers CloudWatch uses for ALB and target group metrics.
output "alb_arn_suffix" {
  value = module.alb.alb_arn_suffix
}

output "target_group_arn_suffix" {
  value = module.alb.target_group_arn_suffix
}
