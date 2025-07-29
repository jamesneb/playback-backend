# Kinesis Data Streams for telemetry ingestion

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
}

variable "project_name" {
  description = "Project name for resource naming"
  type        = string
  default     = "playback-backend"
}

variable "shard_count" {
  description = "Number of shards per stream"
  type        = number
  default     = 1
}

variable "retention_period" {
  description = "Data retention period in hours"
  type        = number
  default     = 24
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}

# Kinesis stream for traces
resource "aws_kinesis_stream" "traces" {
  name             = "${var.project_name}-traces-${var.environment}"
  shard_count      = var.shard_count
  retention_period = var.retention_period

  encryption_type = "KMS"
  kms_key_id      = aws_kms_key.kinesis.arn

  shard_level_metrics = [
    "IncomingRecords",
    "OutgoingRecords",
    "WriteProvisionedThroughputExceeded",
    "ReadProvisionedThroughputExceeded",
    "IncomingBytes",
    "OutgoingBytes",
  ]

  tags = merge(var.tags, {
    Name        = "${var.project_name}-traces-${var.environment}"
    Environment = var.environment
    Component   = "kinesis"
    DataType    = "traces"
  })
}

# Kinesis stream for metrics
resource "aws_kinesis_stream" "metrics" {
  name             = "${var.project_name}-metrics-${var.environment}"
  shard_count      = var.shard_count
  retention_period = var.retention_period

  encryption_type = "KMS"
  kms_key_id      = aws_kms_key.kinesis.arn

  shard_level_metrics = [
    "IncomingRecords",
    "OutgoingRecords",
    "WriteProvisionedThroughputExceeded",
    "ReadProvisionedThroughputExceeded",
    "IncomingBytes",
    "OutgoingBytes",
  ]

  tags = merge(var.tags, {
    Name        = "${var.project_name}-metrics-${var.environment}"
    Environment = var.environment
    Component   = "kinesis"
    DataType    = "metrics"
  })
}

# Kinesis stream for logs
resource "aws_kinesis_stream" "logs" {
  name             = "${var.project_name}-logs-${var.environment}"
  shard_count      = var.shard_count
  retention_period = var.retention_period

  encryption_type = "KMS"
  kms_key_id      = aws_kms_key.kinesis.arn

  shard_level_metrics = [
    "IncomingRecords",
    "OutgoingRecords",
    "WriteProvisionedThroughputExceeded",
    "ReadProvisionedThroughputExceeded",
    "IncomingBytes",
    "OutgoingBytes",
  ]

  tags = merge(var.tags, {
    Name        = "${var.project_name}-logs-${var.environment}"
    Environment = var.environment
    Component   = "kinesis"
    DataType    = "logs"
  })
}

# KMS key for Kinesis encryption
resource "aws_kms_key" "kinesis" {
  description = "KMS key for Kinesis streams encryption in ${var.environment}"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "Enable IAM User Permissions"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action   = "kms:*"
        Resource = "*"
      },
      {
        Sid    = "Allow Kinesis access"
        Effect = "Allow"
        Principal = {
          Service = "kinesis.amazonaws.com"
        }
        Action = [
          "kms:Decrypt",
          "kms:GenerateDataKey"
        ]
        Resource = "*"
      }
    ]
  })

  tags = merge(var.tags, {
    Name        = "${var.project_name}-kinesis-kms-${var.environment}"
    Environment = var.environment
    Component   = "kms"
  })
}

resource "aws_kms_alias" "kinesis" {
  name          = "alias/${var.project_name}-kinesis-${var.environment}"
  target_key_id = aws_kms_key.kinesis.key_id
}

# Data source for current AWS account
data "aws_caller_identity" "current" {}

# CloudWatch alarms for monitoring
resource "aws_cloudwatch_metric_alarm" "traces_incoming_records" {
  alarm_name          = "${var.project_name}-traces-incoming-records-${var.environment}"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "IncomingRecords"
  namespace           = "AWS/Kinesis"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1"
  alarm_description   = "This metric monitors kinesis traces incoming records"
  alarm_actions       = []

  dimensions = {
    StreamName = aws_kinesis_stream.traces.name
  }

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "metrics_incoming_records" {
  alarm_name          = "${var.project_name}-metrics-incoming-records-${var.environment}"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "IncomingRecords"
  namespace           = "AWS/Kinesis"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1"
  alarm_description   = "This metric monitors kinesis metrics incoming records"
  alarm_actions       = []

  dimensions = {
    StreamName = aws_kinesis_stream.metrics.name
  }

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "logs_incoming_records" {
  alarm_name          = "${var.project_name}-logs-incoming-records-${var.environment}"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "IncomingRecords"
  namespace           = "AWS/Kinesis"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1"
  alarm_description   = "This metric monitors kinesis logs incoming records"
  alarm_actions       = []

  dimensions = {
    StreamName = aws_kinesis_stream.logs.name
  }

  tags = var.tags
}