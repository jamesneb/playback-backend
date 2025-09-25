-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

-- Inventory of services/hosts (last seen)
CREATE TABLE IF NOT EXISTS telemetry.inventory_latest
(
  tenant_id String,
  service_name LowCardinality(String),
  host_id LowCardinality(String),
  first_seen_ns Int64,
  last_seen_ns  Int64,
  resource_json Object('json') DEFAULT CAST('{}','Object(\'json\')')
)
ENGINE = ReplacingMergeTree(last_seen_ns)
ORDER BY (tenant_id, service_name, host_id);

-- Rolling edges over normalized spans (5m buckets)
CREATE TABLE IF NOT EXISTS telemetry.edges_5m
(
  tenant_id String,
  ts_bucket DateTime,
  client_service LowCardinality(String),
  server_service LowCardinality(String),
  call_count SimpleAggregateFunction(sum, UInt64),
  error_count SimpleAggregateFunction(sum, UInt64),
  p50_lat_ms AggregateFunction(quantileTiming, Float64),
  p95_lat_ms AggregateFunction(quantileTiming, Float64)
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(ts_bucket)
ORDER BY (tenant_id, ts_bucket, client_service, server_service);

