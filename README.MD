# CS6650 HW6: Product Search Service on ECS with ALB and Auto Scaling

Terraform deploys the HW6 product search service (`src/`) to ECS Fargate behind an Application Load Balancer, with CPU-based auto scaling.

## What gets created
- ECR repository + Docker image (built from `src/` and pushed automatically)
- ECS cluster (Container Insights enabled), task definition (256 CPU / 512 MB), service
- ALB (port 80) → target group (IP, port 8080, health check `/health`, every 30 s, healthy after 2)
- Auto scaling: target tracking on average CPU, 70%, min 2, max 4, cooldowns 300 s
- CloudWatch log group, security groups

## Deploy
Requirements: Terraform, Docker running, AWS credentials in `~/.aws/credentials`.

    cd terraform
    terraform init
    terraform apply -auto-approve

Use the `alb_dns_name` output as the host:

    curl "http://<alb_dns_name>/products/search?q=electro"

## Experiments
Scaling settings are variables, so no file edits are needed:

    terraform apply -auto-approve -var="cpu_target=50"

Other variables: `min_tasks`, `max_tasks`, `scale_out_cooldown`, `scale_in_cooldown`.

## Clean up

    terraform destroy -auto-approve

Service code, Locust tests, and monitoring are in the main `cs6650` repo under `hw06/`. Results are in the submitted PDF report.