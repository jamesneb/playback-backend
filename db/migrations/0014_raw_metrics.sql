-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.metrics_points_raw
(
  tenant_id String,

  service_namespace LowCardinality(String) DEFAULT '',
  service_name      LowCardinality(String) DEFAULT '',
  service_version   LowCardinality(String) DEFAULT '',
  service_instance_id String DEFAULT '',
  host_id           LowCardinality(String) DEFAULT '',

  metric_name   LowCardinality(String),
  unit          LowCardinality(String),
  metric_type   Enum8('GAUGE'=1,'SUM'=2,'HISTOGRAM'=3,'EXP_HISTOGRAM'=4,'SUMMARY'=5),

  time_unix_nano UInt64,
  ts_orig_ns     UInt64 ALIAS time_unix_nano,

  value_f64   Float64 DEFAULT 0.0,
  value_i64   Int64   DEFAULT 0,

  -- histogram (optional)
  h_count     UInt64 DEFAULT 0,
  h_sum       Float64 DEFAULT 0,
  h_bounds    Array(Float64),
  h_bucket_counts Array(UInt64),

  -- labels/attrs (string-only mirror to keep light)
  metric_labels_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  ingested_at DateTime64(9) DEFAULT now64(),
  ingest_ns   UInt64 MATERIALIZED toUnixTimestamp64Nano(ingested_at)
)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(time_unix_nano))
ORDER BY (tenant_id, metric_name, time_unix_nano)
SETTINGS index_granularity = 8192;

ALTER TABLE telemetry.metrics_points_raw
  MODIFY COLUMN metric_name CODEC(ZSTD(6)),
  MODIFY COLUMN unit CODEC(ZSTD(6)),
  MODIFY COLUMN metric_labels_json CODEC(ZSTD(6));

-- MV: explode GAUGE/SUM datapoints
CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_metrics_points_gsum
TO telemetry.metrics_points_raw
AS
SELECT
  tenant_id,
  -- resource → service/host (inline expressions instead of lambda)
  if(indexOf(resource_attr_key,'service.namespace')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.namespace')]) AS service_namespace,
  if(indexOf(resource_attr_key,'service.name')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.name')]) AS service_name,
  if(indexOf(resource_attr_key,'service.version')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.version')]) AS service_version,
  if(indexOf(resource_attr_key,'service.instance.id')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.instance.id')]) AS service_instance_id,
  multiIf(
    indexOf(resource_attr_key,'host.id')>0, resource_attr_v_str[indexOf(resource_attr_key,'host.id')],
    indexOf(resource_attr_key,'host.name')>0, resource_attr_v_str[indexOf(resource_attr_key,'host.name')],
    ''
  ) AS host_id,

  name        AS metric_name,
  unit,
  type        AS metric_type,

  dp_time     AS time_unix_nano,
  dp_val_f    AS value_f64,
  dp_val_i    AS value_i64,
  CAST([] AS Array(Float64))  AS h_bounds,
  CAST([] AS Array(UInt64))   AS h_bucket_counts,
  0           AS h_count,
  0.0         AS h_sum,

  toString(metric_attr_v_str) AS metric_labels_json,
  ingested_at
FROM telemetry.metrics_stage
ARRAY JOIN
  dp_time_unix_nano AS dp_time,
  dp_as_double      AS dp_val_f,
  dp_as_int         AS dp_val_i
WHERE type IN ('GAUGE','SUM');

-- MV: explode HISTOGRAM datapoints
CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_metrics_points_hist
TO telemetry.metrics_points_raw
AS
SELECT
  tenant_id,
  -- resource → service/host (inline expressions instead of lambda)
  if(indexOf(resource_attr_key,'service.namespace')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.namespace')]) AS service_namespace,
  if(indexOf(resource_attr_key,'service.name')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.name')]) AS service_name,
  if(indexOf(resource_attr_key,'service.version')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.version')]) AS service_version,
  if(indexOf(resource_attr_key,'service.instance.id')=0, '', resource_attr_v_str[indexOf(resource_attr_key,'service.instance.id')]) AS service_instance_id,
  multiIf(
    indexOf(resource_attr_key,'host.id')>0, resource_attr_v_str[indexOf(resource_attr_key,'host.id')],
    indexOf(resource_attr_key,'host.name')>0, resource_attr_v_str[indexOf(resource_attr_key,'host.name')],
    ''
  ) AS host_id,

  name AS metric_name,
  unit,
  type AS metric_type,

  h_t AS time_unix_nano,
  0.0 AS value_f64,  -- Replace nan() with 0.0
  0 AS value_i64,
  h_c AS h_count,
  h_s AS h_sum,
  h_b AS h_bounds,
  h_bc AS h_bucket_counts,

  toString(metric_attr_v_str) AS metric_labels_json,
  ingested_at
FROM telemetry.metrics_stage
ARRAY JOIN
  h_time_unix_nano AS h_t,
  h_count          AS h_c,
  h_sum            AS h_s,
  h_bounds         AS h_b,
  h_bucket_counts  AS h_bc
WHERE type = 'HISTOGRAM';

