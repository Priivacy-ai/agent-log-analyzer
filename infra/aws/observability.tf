locals {
  metric_namespace = "ClaudeAnalyzer/${var.environment}"
  alarm_actions    = var.alarm_sns_topic_arn == "" ? [] : [var.alarm_sns_topic_arn]
}

resource "aws_cloudwatch_log_metric_filter" "jobs_completed" {
  name           = "${local.name}-jobs-completed"
  log_group_name = aws_cloudwatch_log_group.app.name
  pattern        = "\"job completed\""

  metric_transformation {
    name      = "JobsCompleted"
    namespace = local.metric_namespace
    value     = "1"
  }
}

resource "aws_cloudwatch_log_metric_filter" "worker_failures" {
  name           = "${local.name}-worker-failures"
  log_group_name = aws_cloudwatch_log_group.app.name
  pattern        = "\"worker process failed\""

  metric_transformation {
    name      = "WorkerFailures"
    namespace = local.metric_namespace
    value     = "1"
  }
}

resource "aws_cloudwatch_log_metric_filter" "sweeps_completed" {
  name           = "${local.name}-sweeps-completed"
  log_group_name = aws_cloudwatch_log_group.app.name
  pattern        = "\"sweep completed\""

  metric_transformation {
    name      = "SweepsCompleted"
    namespace = local.metric_namespace
    value     = "1"
  }
}

resource "aws_cloudwatch_log_metric_filter" "email_event_failures" {
  name           = "${local.name}-email-event-failures"
  log_group_name = aws_cloudwatch_log_group.app.name
  pattern        = "\"email event process failed\""

  metric_transformation {
    name      = "EmailEventFailures"
    namespace = local.metric_namespace
    value     = "1"
  }
}

resource "aws_cloudwatch_log_metric_filter" "email_events_recorded" {
  name           = "${local.name}-email-events-recorded"
  log_group_name = aws_cloudwatch_log_group.app.name
  pattern        = "\"email event recorded\""

  metric_transformation {
    name      = "EmailEventsRecorded"
    namespace = local.metric_namespace
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "alb_elb_5xx" {
  alarm_name          = "${local.name}-alb-elb-5xx"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 60
  statistic           = "Sum"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_ELB_5XX_Count"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    LoadBalancer = aws_lb.api.arn_suffix
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "alb_target_5xx" {
  alarm_name          = "${local.name}-alb-target-5xx"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 60
  statistic           = "Sum"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    LoadBalancer = aws_lb.api.arn_suffix
    TargetGroup  = aws_lb_target_group.api.arn_suffix
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "target_response_time" {
  alarm_name          = "${local.name}-target-response-p95"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  threshold           = 1
  period              = 60
  extended_statistic  = "p95"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "TargetResponseTime"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    LoadBalancer = aws_lb.api.arn_suffix
    TargetGroup  = aws_lb_target_group.api.arn_suffix
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "api_unhealthy_targets" {
  alarm_name          = "${local.name}-api-unhealthy-targets"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  threshold           = 0
  period              = 60
  statistic           = "Average"
  namespace           = "AWS/ApplicationELB"
  metric_name         = "UnHealthyHostCount"
  treat_missing_data  = "breaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    LoadBalancer = aws_lb.api.arn_suffix
    TargetGroup  = aws_lb_target_group.api.arn_suffix
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "api_cpu_high" {
  alarm_name          = "${local.name}-api-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  threshold           = 80
  period              = 60
  statistic           = "Average"
  namespace           = "AWS/ECS"
  metric_name         = "CPUUtilization"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.api.name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "worker_cpu_high" {
  alarm_name          = "${local.name}-worker-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  threshold           = 80
  period              = 60
  statistic           = "Average"
  namespace           = "AWS/ECS"
  metric_name         = "CPUUtilization"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.worker.name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "email_events_cpu_high" {
  alarm_name          = "${local.name}-email-events-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  threshold           = 80
  period              = 60
  statistic           = "Average"
  namespace           = "AWS/ECS"
  metric_name         = "CPUUtilization"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.email_events.name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "queue_age_high" {
  alarm_name          = "${local.name}-queue-age-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  threshold           = 300
  period              = 60
  statistic           = "Maximum"
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateAgeOfOldestMessage"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    QueueName = aws_sqs_queue.jobs.name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "email_events_queue_age_high" {
  alarm_name          = "${local.name}-email-events-queue-age-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  threshold           = 300
  period              = 60
  statistic           = "Maximum"
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateAgeOfOldestMessage"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    QueueName = aws_sqs_queue.email_events.name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "ses_bounces" {
  alarm_name          = "${local.name}-ses-bounces"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 300
  statistic           = "Sum"
  namespace           = "AWS/SES"
  metric_name         = "Bounce"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ConfigurationSet = aws_sesv2_configuration_set.transactional.configuration_set_name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "ses_complaints" {
  alarm_name          = "${local.name}-ses-complaints"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 300
  statistic           = "Sum"
  namespace           = "AWS/SES"
  metric_name         = "Complaint"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ConfigurationSet = aws_sesv2_configuration_set.transactional.configuration_set_name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "ses_rejects" {
  alarm_name          = "${local.name}-ses-rejects"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 300
  statistic           = "Sum"
  namespace           = "AWS/SES"
  metric_name         = "Reject"
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions

  dimensions = {
    ConfigurationSet = aws_sesv2_configuration_set.transactional.configuration_set_name
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "email_event_failures" {
  alarm_name          = "${local.name}-email-event-failures"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 60
  statistic           = "Sum"
  namespace           = local.metric_namespace
  metric_name         = aws_cloudwatch_log_metric_filter.email_event_failures.metric_transformation[0].name
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions
  tags                = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "worker_failures" {
  alarm_name          = "${local.name}-worker-failures"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 60
  statistic           = "Sum"
  namespace           = local.metric_namespace
  metric_name         = aws_cloudwatch_log_metric_filter.worker_failures.metric_transformation[0].name
  treat_missing_data  = "notBreaching"
  alarm_actions       = local.alarm_actions
  tags                = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "sweeper_missing" {
  alarm_name          = "${local.name}-sweeper-missing"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 1
  threshold           = 1
  period              = 900
  statistic           = "Sum"
  namespace           = local.metric_namespace
  metric_name         = aws_cloudwatch_log_metric_filter.sweeps_completed.metric_transformation[0].name
  treat_missing_data  = "breaching"
  alarm_actions       = local.alarm_actions
  tags                = local.common_tags
}

resource "aws_cloudwatch_dashboard" "launch" {
  dashboard_name = "${local.name}-launch"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "text"
        x      = 0
        y      = 0
        width  = 24
        height = 2
        properties = {
          markdown = "# ${local.name} launch dashboard\nPrivacy boundary: operational metrics only; raw uploads and report JSON must not appear in logs."
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 2
        width  = 12
        height = 6
        properties = {
          title  = "ALB traffic and errors"
          region = var.aws_region
          metrics = [
            ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", aws_lb.api.arn_suffix, { stat = "Sum" }],
            [".", "HTTPCode_ELB_5XX_Count", ".", ".", { stat = "Sum" }],
            [".", "HTTPCode_Target_5XX_Count", ".", ".", { stat = "Sum" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 2
        width  = 12
        height = 6
        properties = {
          title  = "Target response time p95"
          region = var.aws_region
          metrics = [
            ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", aws_lb.api.arn_suffix, "TargetGroup", aws_lb_target_group.api.arn_suffix, { stat = "p95" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 8
        width  = 12
        height = 6
        properties = {
          title  = "ECS CPU"
          region = var.aws_region
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", aws_ecs_service.api.name, { label = "api" }],
            [".", ".", ".", ".", "ServiceName", aws_ecs_service.worker.name, { label = "worker" }],
            [".", ".", ".", ".", "ServiceName", aws_ecs_service.email_events.name, { label = "email-events" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 8
        width  = 12
        height = 6
        properties = {
          title  = "Queue depth and age"
          region = var.aws_region
          metrics = [
            ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", aws_sqs_queue.jobs.name, { stat = "Maximum" }],
            [".", "ApproximateNumberOfMessagesNotVisible", ".", ".", { stat = "Maximum" }],
            [".", "ApproximateAgeOfOldestMessage", ".", ".", { stat = "Maximum" }],
            [".", "ApproximateNumberOfMessagesVisible", ".", aws_sqs_queue.email_events.name, { stat = "Maximum", label = "email-events visible" }],
            [".", "ApproximateAgeOfOldestMessage", ".", aws_sqs_queue.email_events.name, { stat = "Maximum", label = "email-events age" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 14
        width  = 12
        height = 6
        properties = {
          title  = "Worker completions and failures"
          region = var.aws_region
          metrics = [
            [local.metric_namespace, aws_cloudwatch_log_metric_filter.jobs_completed.metric_transformation[0].name, { stat = "Sum" }],
            [".", aws_cloudwatch_log_metric_filter.worker_failures.metric_transformation[0].name, { stat = "Sum" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 14
        width  = 12
        height = 6
        properties = {
          title  = "Retention sweeper"
          region = var.aws_region
          metrics = [
            [local.metric_namespace, aws_cloudwatch_log_metric_filter.sweeps_completed.metric_transformation[0].name, { stat = "Sum" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 20
        width  = 12
        height = 6
        properties = {
          title  = "SES transactional outcomes"
          region = var.aws_region
          metrics = [
            ["AWS/SES", "Send", "ConfigurationSet", aws_sesv2_configuration_set.transactional.configuration_set_name, { stat = "Sum" }],
            [".", "Delivery", ".", ".", { stat = "Sum" }],
            [".", "Bounce", ".", ".", { stat = "Sum" }],
            [".", "Complaint", ".", ".", { stat = "Sum" }],
            [".", "Reject", ".", ".", { stat = "Sum" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 20
        width  = 12
        height = 6
        properties = {
          title  = "SES event worker"
          region = var.aws_region
          metrics = [
            [local.metric_namespace, aws_cloudwatch_log_metric_filter.email_events_recorded.metric_transformation[0].name, { stat = "Sum" }],
            [".", aws_cloudwatch_log_metric_filter.email_event_failures.metric_transformation[0].name, { stat = "Sum" }]
          ]
        }
      }
    ]
  })
}
