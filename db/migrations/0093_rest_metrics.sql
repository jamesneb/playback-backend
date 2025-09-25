-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.rest_metrics_stage
(
  tenant_id String,

  service_namespace   String DEFAULT '',
  service_name        String,
  service_version     String DEFAULT '',
  service_instance_id String DEFAULT '',
  host_id             String DEFAULT '',

  metric_name  String,
  unit         String DEFAULT '',
  metric_type  Enum8('GAUGE'=1,'SUM'=2,'HISTOGRAM'=3,'EXP_HISTOGRAM'=4,'SUMMARY'=5),

  time_unix_nano UInt64,

  -- GAUGE/SUM values
  value_f64   Float64        DEFAULT 0.0,
  value_i64   Int64          DEFAULT 0,

  -- HISTOGRAM
  h_count     UInt64         DEFAULT 0,
  h_sum       Float64        DEFAULT 0,
  h_bounds    Array(Float64) DEFAULT CAST([] AS Array(Float64)),
  h_bucket_counts Array(UInt64) DEFAULT CAST([] AS Array(UInt64)),

  -- EXP_HISTOGRAM
  eh_scale       Int32         DEFAULT 0,
  eh_zero_count  UInt64        DEFAULT 0,
  eh_pos_offset  Int32         DEFAULT 0,
  eh_pos_counts  Array(UInt64) DEFAULT CAST([] AS Array(UInt64)),
  eh_neg_offset  Int32         DEFAULT 0,
  eh_neg_counts  Array(UInt64) DEFAULT CAST([] AS Array(UInt64)),

  -- SUMMARY
  s_count      UInt64         DEFAULT 0,
  s_sum        Float64        DEFAULT 0,
  s_quantiles  Array(Float64) DEFAULT CAST([] AS Array(Float64)),
  s_values     Array(Float64) DEFAULT CAST([] AS Array(Float64)),

  metric_labels_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  ingested_at DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDate(intDiv(time_unix_nano, 86400000000000))
ORDER BY (tenant_id, metric_name, time_unix_nano)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_rest_to_metrics_points
TO telemetry.metrics_points_raw
AS
SELECT
  tenant_id,
  service_namespace, service_name, service_version, service_instance_id, host_id,

  metric_name, unit, metric_type,
  time_unix_nano,

  value_f64, value_i64,

  h_count, h_sum, h_bounds, h_bucket_counts,

  eh_scale, eh_zero_count, eh_pos_offset, eh_pos_counts, eh_neg_offset, eh_neg_counts,

  s_count, s_sum, s_quantiles, s_values,

  metric_labels_json,
  ingested_at
FROM telemetry.rest_metrics_stage;

