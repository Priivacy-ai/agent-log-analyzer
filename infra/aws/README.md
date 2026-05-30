# AWS Infrastructure

Terraform for the production serverless deployment.

What it creates:

- Private S3 static bucket served by CloudFront.
- API Gateway HTTP API backed by the API Lambda.
- SQS-triggered Lambda worker.
- EventBridge-triggered Lambda sweeper.
- SQS-triggered Lambda SES event recorder.
- Private S3 buckets for raw uploads and sanitized reports, with encryption, public-access blocks, and one-day lifecycle backstops.
- SQS queue and DynamoDB job table.
- SES transactional configuration set and event queue.

Prepare:

```sh
cd infra/aws
terraform init
terraform validate
terraform plan
```

Lambda/static deploy:

```sh
AWS_PROFILE=claude-analyzer-prod AWS_REGION=us-east-1 ./scripts/build-lambda-zips.sh
AWS_PROFILE=claude-analyzer-prod AWS_REGION=us-east-1 terraform -chdir=infra/aws apply
AWS_PROFILE=claude-analyzer-prod AWS_REGION=us-east-1 ./scripts/deploy-serverless-static.sh
```

Production notes:

- Launch hostname is `analyzer.spec-kitty.ai`.
- DNS for `spec-kitty.ai` is managed in Namecheap, not Route 53.
- `analyzer.spec-kitty.ai` must CNAME to the CloudFront output
  `cloudfront_domain_name`.
- Keep `force_destroy_buckets=false` in production.
- The public upload UX is Claude/prompt/curl only. There is no browser multipart upload form.
- Current cheapest AWS-native footprint intentionally has no VPC, NAT gateway,
  VPC endpoints, ALB, ECS services, regional WAF, CloudWatch dashboard, or
  custom metric alarms.

Transactional email:

- SES is the production Terraform provider (`email_provider=ses`). The current
  AWS account has sending enabled with the SES sandbox quota: 200 emails/day and
  1 email/second. Sandbox mode can send only to verified recipients/domains, so
  request SES production access before relying on arbitrary customer recipient
  delivery.
- Postmark was removed from the production AWS footprint. Reintroducing an
  external email provider means adding secret delivery and outbound internet
  access intentionally, not by restoring the old NAT-backed ECS stack.

Admin usage stats:

The API can expose aggregate usage stats at `/api/admin/usage-stats`. Set `admin_token_sha256` to the SHA-256 hex digest of a strong bearer token; do not pass or commit the raw token through Terraform. Set `usage_hash_salt` to enable privacy-preserving unique-client counts in usage logs.
