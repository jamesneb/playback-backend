-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.logs_stage
(
  tenant_id String  DEFAULT '',

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

  /* Log fields */
  time_unix_nano          UInt64,
  observed_time_unix_nano UInt64,
  severity_number         UInt8 DEFAULT 0,
  severity_text           String DEFAULT '',
  body_str                String DEFAULT '',
  body_json               Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  /* Trace correlation */
  trace_id_bin  FixedString(16) DEFAULT unhex('00000000000000000000000000000000'),
  span_id_bin   FixedString(8)  DEFAULT unhex('0000000000000000'),
  trace_id_hex  String ALIAS lower(hex(trace_id_bin)),
  span_id_hex   String ALIAS lower(hex(span_id_bin)),

  /* Attributes */
  attr_key     Array(String),
  attr_v_str   Array(String),
  attr_v_i64   Array(Int64),
  attr_v_f64   Array(Float64),
  attr_v_bool  Array(UInt8),
  attr_v_bytes Array(String),

  ingested_at DateTime64(9) DEFAULT now64()

)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(time_unix_nano))
ORDER BY (tenant_id, time_unix_nano, trace_id_bin)
SETTINGS index_granularity = 8192;

ALTER TABLE telemetry.logs_stage
  MODIFY COLUMN body_str CODEC(ZSTD(6));


