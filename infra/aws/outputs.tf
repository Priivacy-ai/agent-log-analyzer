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

output "serverless_api_endpoint" {
  description = "HTTP API endpoint used by the serverless API Lambda."
  value       = aws_apigatewayv2_api.http.api_endpoint
}

output "cloudfront_domain_name" {
  description = "CloudFront distribution domain for the serverless site."
  value       = aws_cloudfront_distribution.site.domain_name
}

output "static_bucket" {
  description = "Static site bucket served through CloudFront."
  value       = aws_s3_bucket.static.bucket
}
