-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.rest_spans_stage
(
  tenant_id String,

  service_namespace   String DEFAULT '',
  service_name        String,
  service_version     String DEFAULT '',
  service_instance_id String DEFAULT '',
  host_id             String DEFAULT '',

  trace_id_hex        String,
  span_id_hex         String,
  parent_span_id_hex  String DEFAULT '',

  name                String,
  kind                Enum8('UNSPECIFIED'=0,'INTERNAL'=1,'SERVER'=2,'CLIENT'=3,'PRODUCER'=4,'CONSUMER'=5) DEFAULT 'UNSPECIFIED',
  start_time_unix_nano UInt64,
  end_time_unix_nano   UInt64,

  status_code         UInt8  DEFAULT 0,
  status_message      String DEFAULT '',

  resource_json       Object('json') DEFAULT CAST('{}','Object(\'json\')'),
  attributes_json     Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  ingested_at         DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDate(intDiv(start_time_unix_nano, 86400000000000))
ORDER BY (tenant_id, service_name, host_id, start_time_unix_nano)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_rest_to_spans_raw
TO telemetry.spans_raw
AS
SELECT
  tenant_id,
  service_namespace, service_name, service_version, service_instance_id, host_id,

  if(length(trace_id_hex)=32, unhex(trace_id_hex), unhex('00000000000000000000000000000000')) AS trace_id_bin,
  if(length(span_id_hex)=16,  unhex(span_id_hex),  unhex('0000000000000000'))                 AS span_id_bin,
  if(length(parent_span_id_hex)=16, unhex(parent_span_id_hex), unhex('0000000000000000'))      AS parent_span_id_bin,

  name, kind, start_time_unix_nano, end_time_unix_nano,
  status_code, status_message,
  ingested_at,

  resource_json,
  attributes_json
FROM telemetry.rest_spans_stage;

