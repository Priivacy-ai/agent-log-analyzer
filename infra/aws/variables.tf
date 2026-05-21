variable "aws_region" {
  type        = string
  description = "AWS region for launch infrastructure."
  default     = "us-east-1"
}

variable "project" {
  type        = string
  description = "Resource name prefix."
  default     = "claude-analyzer"
}

variable "environment" {
  type        = string
  description = "Environment name."
  default     = "prod"
}

variable "container_image" {
  type        = string
  description = "API/worker/sweeper container image. Defaults to this stack's ECR repository latest tag when empty."
  default     = ""
}

variable "api_desired_count" {
  type        = number
  description = "Initial API task count."
  default     = 2
}

variable "worker_desired_count" {
  type        = number
  description = "Initial worker task count."
  default     = 4
}

variable "email_events_desired_count" {
  type        = number
  description = "Initial SES email event worker task count."
  default     = 1
}

variable "max_queue_depth" {
  type        = number
  description = "Queue depth where API load-sheds uploads."
  default     = 1000
}

variable "email_provider" {
  type        = string
  description = "Transactional email provider for confirmation and full-scan delivery. Supported values: ses, postmark, or empty to log only."
  default     = "ses"
}

variable "email_from" {
  type        = string
  description = "Verified sender address for transactional email."
  default     = "noreply@spec-kitty.ai"
}

variable "postmark_message_stream" {
  type        = string
  description = "Postmark message stream ID for transactional email. Defaults to Postmark's outbound transactional stream."
  default     = "outbound"
}

variable "postmark_server_token_secret_arn" {
  type        = string
  description = "Optional AWS Secrets Manager ARN containing POSTMARK_SERVER_TOKEN. Required only when email_provider=postmark."
  default     = ""
}

variable "certificate_arn" {
  type        = string
  description = "Optional ACM certificate ARN for HTTPS listener."
  default     = ""
}

variable "force_destroy_buckets" {
  type        = bool
  description = "Allow Terraform destroy to delete non-empty buckets. Keep false for production."
  default     = false
}

variable "alarm_sns_topic_arn" {
  type        = string
  description = "Optional SNS topic ARN for CloudWatch alarm notifications. Leave empty to create alarms without actions."
  default     = ""
}

variable "waf_rate_limit_per_5m" {
  type        = number
  description = "Maximum requests per source IP per five-minute WAF window."
  default     = 2000
}
