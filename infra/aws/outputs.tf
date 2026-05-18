output "ecr_repository_url" {
  description = "Push the application image here before running ECS services."
  value       = aws_ecr_repository.app.repository_url
}

output "alb_dns_name" {
  description = "Public API/UI load balancer DNS name."
  value       = aws_lb.api.dns_name
}

output "upload_bucket" {
  description = "Raw upload quarantine bucket."
  value       = aws_s3_bucket.uploads.bucket
}

output "report_bucket" {
  description = "Sanitized report bucket."
  value       = aws_s3_bucket.reports.bucket
}

output "job_queue_url" {
  description = "SQS analysis queue URL."
  value       = aws_sqs_queue.jobs.url
}

output "job_table" {
  description = "DynamoDB job table name."
  value       = aws_dynamodb_table.jobs.name
}

output "cloudwatch_dashboard_name" {
  description = "Launch operations dashboard."
  value       = aws_cloudwatch_dashboard.launch.dashboard_name
}

output "waf_web_acl_arn" {
  description = "Regional WAF web ACL associated with the public ALB."
  value       = aws_wafv2_web_acl.alb.arn
}
