# AWS Infrastructure

Terraform scaffold for the first production deployment. Do not apply this until the Docker-local and LocalStack gates are green and the AWS account is ready.

What it creates:

- ECR repository for the app image.
- VPC with public ALB subnets and private ECS task subnets.
- VPC endpoints for S3, DynamoDB, SQS, ECR, and CloudWatch Logs; no NAT gateway is required.
- Private S3 buckets for raw uploads and sanitized reports, with encryption, public-access blocks, and one-day lifecycle backstops.
- SQS queue and DynamoDB job table.
- ECS Fargate API and worker services.
- Scheduled Fargate sweeper every five minutes to enforce the 15-minute upload/report TTL.

Prepare:

```sh
cd infra/aws
terraform init
terraform validate
terraform plan
```

Image flow:

```sh
aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin "$(terraform output -raw ecr_repository_url | cut -d/ -f1)"

docker build -t "$(terraform output -raw ecr_repository_url):latest" ../..
docker push "$(terraform output -raw ecr_repository_url):latest"
```

Production notes:

- Pass `certificate_arn` to enable the HTTPS listener.
- Launch hostname is `analyzer.spec-kitty.ai`.
- Keep the previous `claude-code.spec-kitty.ai` hostname only as a compatibility redirect to `analyzer.spec-kitty.ai`; do not use it in public launch copy.
- DNS for `spec-kitty.ai` is managed in Namecheap, not Route 53. Add the `analyzer` app CNAME and ACM validation CNAME there.
- When `certificate_arn` is set, the ALB HTTP listener redirects to HTTPS.
- When applying only TLS, also pass the current `container_image` value to avoid unintentionally changing ECS task definitions back to `:latest`.
- Keep `force_destroy_buckets=false` in production.
- Put CloudFront and WAF in front of the ALB before a public launch.
- The public upload UX is Claude/prompt/curl only. There is no browser multipart upload form.
- Scale tokenized upload traffic by isolating the API upload path behind the ALB, keeping workers asynchronous, and autoscaling API tasks independently from workers.
