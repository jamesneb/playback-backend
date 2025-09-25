CREATE TABLE IF NOT EXISTS telemetry.metrics_stage
(

  tenant_id String DEFAULT '',
  /* Resource attrs */
  resource_attr_key     Array(String),
  resource_attr_v_str   Array(String),
  resource_attr_v_i64   Array(Int64),
  resource_attr_v_f64   Array(Float64),
  resource_attr_v_bool  Array(UInt8),
  resource_attr_v_bytes Array(String),

  /* Scope */
  scope_name    String DEFAULT '',
  scope_version String DEFAULT '',

  /* Descriptors */
  name  String,
  unit  String DEFAULT '',
  type  Enum8('GAUGE'=1,'SUM'=2,'HISTOGRAM'=3,'EXP_HISTOGRAM'=4,'SUMMARY'=5),

  /* Labels */
  metric_attr_key   Array(String),
  metric_attr_v_str Array(String),

  /* GAUGE/SUM data points */
  dp_time_unix_nano Array(UInt64),
  dp_as_double      Array(Float64),
  dp_as_int         Array(Int64),

  /* HISTOGRAM datapoints */
  h_time_unix_nano      Array(UInt64),
  h_count               Array(UInt64),
  h_sum                 Array(Float64),
  h_bounds              Array(Array(Float64)),  -- explicit bounds
  h_bucket_counts       Array(Array(UInt64)),

  ingested_at DateTime64(9) DEFAULT now64()

)
ENGINE = MergeTree
PARTITION BY toDate(ingested_at)
ORDER BY (tenant_id, name, ingested_at)
SETTINGS index_granularity = 8192;

ALTER TABLE telemetry.metrics_stage
  MODIFY COLUMN name CODEC(ZSTD(6)),
  MODIFY COLUMN unit CODEC(ZSTD(6));
