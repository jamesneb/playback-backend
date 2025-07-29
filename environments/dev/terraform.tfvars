# Development Environment Configuration

# Basic configuration
environment    = "dev"
project_name   = "playback-backend"
aws_region     = "us-east-1"

# Networking
vpc_cidr           = "10.0.0.0/16"
availability_zones = ["us-east-1a", "us-east-1b"]
public_subnets     = ["10.0.1.0/24", "10.0.2.0/24"]
private_subnets    = ["10.0.10.0/24", "10.0.20.0/24"]

# Kinesis configuration (minimal for dev)
kinesis_shard_count      = 1
kinesis_retention_period = 24

# ClickHouse configuration (small instance for dev)
clickhouse_instance_type = "t3.medium"
clickhouse_instance_count = 1
clickhouse_storage_size = 20

# Redis configuration (small instance for dev)
redis_node_type = "cache.t3.micro"
redis_num_cache_nodes = 1

# Lambda configuration
lambda_memory_size = 256
lambda_timeout = 60
lambda_reserved_concurrency = 10

# Auto-scaling configuration (conservative for dev)
min_capacity = 1
max_capacity = 3
target_cpu_utilization = 70

# Monitoring and logging
enable_detailed_monitoring = true
log_retention_in_days = 7

# Cost optimization for dev
enable_spot_instances = true
enable_scheduled_scaling = false

# Tags
tags = {
  Environment = "dev"
  Project     = "playback-backend"
  Owner       = "development-team"
  CostCenter  = "engineering"
  Terraform   = "true"
}