locals {
  lambda_zip_dir  = "${path.module}/../../.data/lambda"
  static_bucket   = "${local.name}-static-${random_id.suffix.hex}"
  lambda_base_env = [for item in local.env : item if item.name != "AWS_REGION"]
  lambda_env = concat(local.lambda_base_env, local.admin_usage_env, [
    { name = "CLAUDE_ANALYZER_SES_CONFIGURATION_SET", value = aws_sesv2_configuration_set.transactional.configuration_set_name },
    { name = "CLAUDE_ANALYZER_PUBLIC_BASE_URL", value = "https://analyzer.spec-kitty.ai" }
  ])
}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

resource "aws_s3_bucket" "static" {
  bucket        = local.static_bucket
  force_destroy = var.force_destroy_buckets
  tags          = local.common_tags
}

resource "aws_s3_bucket_public_access_block" "static" {
  bucket                  = aws_s3_bucket.static.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "static" {
  bucket = aws_s3_bucket.static.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_cloudfront_origin_access_control" "static" {
  name                              = "${local.name}-static"
  description                       = "Private static site bucket access"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_function" "static_index_rewrite" {
  name    = "${local.name}-static-index-rewrite"
  runtime = "cloudfront-js-2.0"
  comment = "Map clean static URLs to index.html objects."
  publish = true
  code    = <<JS
function handler(event) {
  var request = event.request;
  var uri = request.uri;
  if (uri.endsWith('/')) {
    request.uri = uri + 'index.html';
  } else if (!uri.includes('.')) {
    request.uri = uri + '/index.html';
  }
  return request;
}
JS
}

resource "aws_s3_bucket_policy" "static_cloudfront" {
  bucket = aws_s3_bucket.static.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowCloudFrontRead"
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = "s3:GetObject"
      Resource  = "${aws_s3_bucket.static.arn}/*"
      Condition = {
        StringEquals = {
          "AWS:SourceArn" = aws_cloudfront_distribution.site.arn
        }
      }
    }]
  })
}

resource "aws_iam_role" "lambda" {
  name = "${local.name}-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "lambda.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })

  tags = local.common_tags
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "lambda_app" {
  name = "${local.name}-lambda-app"
  role = aws_iam_role.lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
        Resource = [
          aws_s3_bucket.uploads.arn,
          "${aws_s3_bucket.uploads.arn}/*",
          aws_s3_bucket.reports.arn,
          "${aws_s3_bucket.reports.arn}/*"
        ]
      },
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage", "sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = [aws_sqs_queue.jobs.arn, aws_sqs_queue.email_events.arn]
      },
      {
        Effect   = "Allow"
        Action   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:Scan"]
        Resource = aws_dynamodb_table.jobs.arn
      },
      {
        Effect   = "Allow"
        Action   = ["ses:SendEmail"]
        Resource = "*"
      }
    ]
  })
}

resource "aws_lambda_function" "api" {
  function_name    = "${local.name}-api"
  role             = aws_iam_role.lambda.arn
  filename         = "${local.lambda_zip_dir}/api.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/api.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 512
  timeout          = 30

  environment {
    variables = { for item in local.lambda_env : item.name => item.value }
  }

  tags = local.common_tags
}

resource "aws_lambda_function" "worker" {
  function_name    = "${local.name}-worker"
  role             = aws_iam_role.lambda.arn
  filename         = "${local.lambda_zip_dir}/worker.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/worker.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 1024
  timeout          = 180

  environment {
    variables = { for item in local.lambda_base_env : item.name => item.value }
  }

  tags = local.common_tags
}

resource "aws_lambda_function" "sweeper" {
  function_name    = "${local.name}-sweeper"
  role             = aws_iam_role.lambda.arn
  filename         = "${local.lambda_zip_dir}/sweeper.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/sweeper.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256
  timeout          = 180

  environment {
    variables = merge(
      { for item in local.lambda_base_env : item.name => item.value },
      {
        CLAUDE_ANALYZER_UPLOAD_TTL = "15m"
        CLAUDE_ANALYZER_REPORT_TTL = "0"
      }
    )
  }

  tags = local.common_tags
}

resource "aws_lambda_function" "email_events" {
  function_name    = "${local.name}-email-events"
  role             = aws_iam_role.lambda.arn
  filename         = "${local.lambda_zip_dir}/email-events.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/email-events.zip")
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256
  timeout          = 90

  environment {
    variables = { for item in local.lambda_base_env : item.name => item.value }
  }

  tags = local.common_tags
}

resource "aws_lambda_event_source_mapping" "worker_jobs" {
  event_source_arn = aws_sqs_queue.jobs.arn
  function_name    = aws_lambda_function.worker.arn
  batch_size       = 1
  enabled          = true
}

resource "aws_lambda_event_source_mapping" "email_events" {
  event_source_arn = aws_sqs_queue.email_events.arn
  function_name    = aws_lambda_function.email_events.arn
  batch_size       = 5
  enabled          = true
}

resource "aws_cloudwatch_event_target" "serverless_sweeper" {
  rule = aws_cloudwatch_event_rule.sweeper.name
  arn  = aws_lambda_function.sweeper.arn
}

resource "aws_lambda_permission" "allow_sweeper_events" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.sweeper.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.sweeper.arn
}

resource "aws_apigatewayv2_api" "http" {
  name          = "${local.name}-http"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "api" {
  api_id                 = aws_apigatewayv2_api.http.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.api.invoke_arn
  payload_format_version = "2.0"
  timeout_milliseconds   = 30000
}

resource "aws_apigatewayv2_route" "api_proxy" {
  api_id    = aws_apigatewayv2_api.http.id
  route_key = "ANY /{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.api.id}"
}

resource "aws_apigatewayv2_route" "api_root" {
  api_id    = aws_apigatewayv2_api.http.id
  route_key = "ANY /"
  target    = "integrations/${aws_apigatewayv2_integration.api.id}"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.http.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.app.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      responseLength = "$context.responseLength"
    })
  }
}

resource "aws_lambda_permission" "allow_http_api" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.http.execution_arn}/*/*"
}

resource "aws_cloudfront_distribution" "site" {
  enabled             = true
  comment             = "${local.name} serverless site"
  default_root_object = "index.html"
  aliases             = var.certificate_arn == "" ? [] : ["analyzer.spec-kitty.ai"]
  price_class         = "PriceClass_100"

  origin {
    origin_id                = "static"
    domain_name              = aws_s3_bucket.static.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.static.id
  }

  origin {
    origin_id   = "api"
    domain_name = "${aws_apigatewayv2_api.http.id}.execute-api.${var.aws_region}.amazonaws.com"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "static"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = data.aws_cloudfront_cache_policy.caching_optimized.id
    compress               = true

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.static_index_rewrite.arn
    }
  }

  dynamic "ordered_cache_behavior" {
    for_each = toset(["/api/*", "/r/*", "/email/confirm/*", "/health", "/healthz"])
    content {
      path_pattern             = ordered_cache_behavior.value
      target_origin_id         = "api"
      viewer_protocol_policy   = "redirect-to-https"
      allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
      cached_methods           = ["GET", "HEAD"]
      cache_policy_id          = data.aws_cloudfront_cache_policy.caching_disabled.id
      origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
      compress                 = true
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = var.certificate_arn == "" ? null : var.certificate_arn
    cloudfront_default_certificate = var.certificate_arn == "" ? true : null
    minimum_protocol_version       = var.certificate_arn == "" ? null : "TLSv1.2_2021"
    ssl_support_method             = var.certificate_arn == "" ? null : "sni-only"
  }

  tags = local.common_tags
}
