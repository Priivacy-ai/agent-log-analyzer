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
  description = "Postmark message stream used when email_provider=postmark."
  default     = "outbound"
}

variable "postmark_server_token_secret_arn" {
  type        = string
  description = "Optional AWS Secrets Manager secret ARN containing the Postmark server token."
  default     = ""
}

variable "certificate_arn" {
  type        = string
  description = "Optional ACM certificate ARN for the CloudFront custom hostname."
  default     = ""
}

variable "force_destroy_buckets" {
  type        = bool
  description = "Allow Terraform destroy to delete non-empty buckets. Keep false for production."
  default     = false
}

variable "admin_token_sha256" {
  type        = string
  description = "Optional SHA-256 hex digest for the bearer token that can read admin analytics and email export endpoints."
  default     = ""
  sensitive   = true
}

variable "usage_hash_salt" {
  type        = string
  description = "Optional salt used to hash client IPs in usage logs. Leave empty to disable client hashes."
  default     = ""
  sensitive   = true
}
