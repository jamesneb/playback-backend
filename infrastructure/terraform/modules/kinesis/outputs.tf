# Output values for the Kinesis module

output "traces_stream_name" {
  description = "Name of the traces Kinesis stream"
  value       = aws_kinesis_stream.traces.name
}

output "traces_stream_arn" {
  description = "ARN of the traces Kinesis stream"
  value       = aws_kinesis_stream.traces.arn
}

output "metrics_stream_name" {
  description = "Name of the metrics Kinesis stream"
  value       = aws_kinesis_stream.metrics.name
}

output "metrics_stream_arn" {
  description = "ARN of the metrics Kinesis stream"
  value       = aws_kinesis_stream.metrics.arn
}

output "logs_stream_name" {
  description = "Name of the logs Kinesis stream"
  value       = aws_kinesis_stream.logs.name
}

output "logs_stream_arn" {
  description = "ARN of the logs Kinesis stream"
  value       = aws_kinesis_stream.logs.arn
}

output "kms_key_id" {
  description = "KMS key ID for Kinesis encryption"
  value       = aws_kms_key.kinesis.key_id
}

output "kms_key_arn" {
  description = "KMS key ARN for Kinesis encryption"
  value       = aws_kms_key.kinesis.arn
}

output "stream_names" {
  description = "Map of all stream names"
  value = {
    traces  = aws_kinesis_stream.traces.name
    metrics = aws_kinesis_stream.metrics.name
    logs    = aws_kinesis_stream.logs.name
  }
}

output "stream_arns" {
  description = "Map of all stream ARNs"
  value = {
    traces  = aws_kinesis_stream.traces.arn
    metrics = aws_kinesis_stream.metrics.arn
    logs    = aws_kinesis_stream.logs.arn
  }
}