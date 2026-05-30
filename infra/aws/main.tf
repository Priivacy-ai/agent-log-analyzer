data "aws_caller_identity" "current" {}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  name          = "${var.project}-${var.environment}"
  upload_bucket = "${local.name}-uploads-${random_id.suffix.hex}"
  report_bucket = "${local.name}-reports-${random_id.suffix.hex}"
  common_tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
  }
  admin_usage_env = concat(
    var.admin_token_sha256 == "" ? [] : [
      { name = "CLAUDE_ANALYZER_ADMIN_TOKEN_SHA256", value = var.admin_token_sha256 }
    ],
    var.usage_hash_salt == "" ? [] : [
      { name = "CLAUDE_ANALYZER_USAGE_HASH_SALT", value = var.usage_hash_salt }
    ]
  )
}

resource "aws_s3_bucket" "uploads" {
  bucket        = local.upload_bucket
  force_destroy = var.force_destroy_buckets
  tags          = local.common_tags
}

resource "aws_s3_bucket" "reports" {
  bucket        = local.report_bucket
  force_destroy = var.force_destroy_buckets
  tags          = local.common_tags
}

resource "aws_s3_bucket_public_access_block" "uploads" {
  bucket                  = aws_s3_bucket.uploads.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_public_access_block" "reports" {
  bucket                  = aws_s3_bucket.reports.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "reports" {
  bucket = aws_s3_bucket.reports.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    id     = "backstop-expire-raw-uploads"
    status = "Enabled"

    expiration {
      days = 1
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "reports" {
  bucket = aws_s3_bucket.reports.id

  rule {
    id     = "expire-cookie-free-usage-events"
    status = "Enabled"

    filter {
      prefix = "usage/events/"
    }

    expiration {
      days = 90
    }
  }

  rule {
    id     = "expire-aggregate-analytics-events"
    status = "Enabled"

    filter {
      prefix = "analytics/events/"
    }

    expiration {
      days = 90
    }
  }
}

resource "aws_sqs_queue" "jobs" {
  name                       = "${local.name}-jobs"
  visibility_timeout_seconds = 180
  message_retention_seconds  = 86400
  sqs_managed_sse_enabled    = true
  tags                       = local.common_tags
}

resource "aws_sqs_queue" "email_events" {
  name                       = "${local.name}-email-events"
  visibility_timeout_seconds = 90
  message_retention_seconds  = 1209600
  sqs_managed_sse_enabled    = true
  tags                       = local.common_tags
}

resource "aws_sns_topic" "ses_events" {
  name = "${local.name}-ses-events"
  tags = local.common_tags
}

resource "aws_sns_topic_policy" "ses_events" {
  arn = aws_sns_topic.ses_events.arn

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "ses.amazonaws.com"
      }
      Action   = "SNS:Publish"
      Resource = aws_sns_topic.ses_events.arn
      Condition = {
        StringEquals = {
          "AWS:SourceAccount" = data.aws_caller_identity.current.account_id
        }
      }
    }]
  })
}

resource "aws_sqs_queue_policy" "email_events" {
  queue_url = aws_sqs_queue.email_events.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "sns.amazonaws.com"
      }
      Action   = "sqs:SendMessage"
      Resource = aws_sqs_queue.email_events.arn
      Condition = {
        ArnEquals = {
          "aws:SourceArn" = aws_sns_topic.ses_events.arn
        }
      }
    }]
  })
}

resource "aws_sns_topic_subscription" "ses_events_email_queue" {
  topic_arn = aws_sns_topic.ses_events.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.email_events.arn
}

resource "aws_sesv2_account_suppression_attributes" "account" {
  suppressed_reasons = ["BOUNCE", "COMPLAINT"]
}

resource "aws_sesv2_configuration_set" "transactional" {
  configuration_set_name = "${local.name}-transactional"
}

resource "aws_sesv2_configuration_set_event_destination" "transactional_sns" {
  configuration_set_name = aws_sesv2_configuration_set.transactional.configuration_set_name
  event_destination_name = "${local.name}-sns"

  event_destination {
    enabled = true
    matching_event_types = [
      "SEND",
      "DELIVERY",
      "BOUNCE",
      "COMPLAINT",
      "REJECT",
      "RENDERING_FAILURE",
      "DELIVERY_DELAY"
    ]

    sns_destination {
      topic_arn = aws_sns_topic.ses_events.arn
    }
  }
}

resource "aws_dynamodb_table" "jobs" {
  name         = "${local.name}-jobs"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${local.name}"
  retention_in_days = 90
  tags              = local.common_tags
}

locals {
  env = [
    { name = "AWS_REGION", value = var.aws_region },
    { name = "CLAUDE_ANALYZER_BACKEND", value = "aws" },
    { name = "CLAUDE_ANALYZER_UPLOAD_BUCKET", value = aws_s3_bucket.uploads.bucket },
    { name = "CLAUDE_ANALYZER_REPORT_BUCKET", value = aws_s3_bucket.reports.bucket },
    { name = "CLAUDE_ANALYZER_JOB_TABLE", value = aws_dynamodb_table.jobs.name },
    { name = "CLAUDE_ANALYZER_JOB_QUEUE_URL", value = aws_sqs_queue.jobs.url },
    { name = "CLAUDE_ANALYZER_EMAIL_PROVIDER", value = var.email_provider },
    { name = "CLAUDE_ANALYZER_EMAIL_FROM", value = var.email_from }
  ]
}

resource "aws_cloudwatch_event_rule" "sweeper" {
  name                = "${local.name}-sweeper"
  schedule_expression = "rate(5 minutes)"
  tags                = local.common_tags
}
