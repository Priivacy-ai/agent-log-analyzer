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

variable "max_queue_depth" {
  type        = number
  description = "Queue depth where API load-sheds uploads."
  default     = 1000
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
