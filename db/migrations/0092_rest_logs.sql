-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.rest_logs_stage
(
  tenant_id String,

  service_namespace   String DEFAULT '',
  service_name        String,
  service_version     String DEFAULT '',
  service_instance_id String DEFAULT '',
  host_id             String DEFAULT '',

  time_unix_nano          UInt64,
  observed_time_unix_nano UInt64 DEFAULT 0,

  severity_number UInt8  DEFAULT 0,
  severity_text   String DEFAULT '',

  body_str        String DEFAULT '',
  body_json       Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  trace_id_hex    String DEFAULT '',
  span_id_hex     String DEFAULT '',

  attributes_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  ingested_at     DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDate(intDiv(time_unix_nano, 86400000000000))
ORDER BY (tenant_id, service_name, host_id, time_unix_nano)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_rest_to_logs_raw
TO telemetry.logs_raw
AS
SELECT
  tenant_id,
  service_namespace, service_name, service_version, service_instance_id, host_id,
  time_unix_nano, observed_time_unix_nano,
  severity_number, severity_text, body_str, body_json,
  if(length(trace_id_hex)=32, unhex(trace_id_hex), unhex('00000000000000000000000000000000')) AS trace_id_bin,
  if(length(span_id_hex)=16,  unhex(span_id_hex),  unhex('0000000000000000'))                 AS span_id_bin,
  attributes_json,
  ingested_at
FROM telemetry.rest_logs_stage;

