-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.logs_raw
(

 tenant_id String,

  service_namespace  LowCardinality(String) DEFAULT '',
  service_name       LowCardinality(String) DEFAULT '',
  service_version    LowCardinality(String) DEFAULT '',
  service_instance_id String DEFAULT '',
  host_id            LowCardinality(String) DEFAULT '',

  time_unix_nano UInt64,
  ts_orig_ns     UInt64 ALIAS time_unix_nano,
  observed_time_unix_nano UInt64 DEFAULT 0,

  severity_number UInt8  DEFAULT 0,
  severity_text   LowCardinality(String) DEFAULT '',
  body_str        String DEFAULT '',
  body_json       Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  trace_id_bin FixedString(16),
  span_id_bin  FixedString(8),
  trace_id_hex String ALIAS lower(hex(trace_id_bin)),
  span_id_hex  String ALIAS lower(hex(span_id_bin)),

  attributes_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),

  ingested_at DateTime64(9) DEFAULT now64(),
  ingest_ns   UInt64 MATERIALIZED toUnixTimestamp64Nano(ingested_at)
)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(time_unix_nano))
ORDER BY (tenant_id, service_name, host_id, time_unix_nano)
SETTINGS index_granularity = 8192;

ALTER TABLE telemetry.logs_raw
  MODIFY COLUMN body_str CODEC(ZSTD(6)),
  MODIFY COLUMN attributes_json CODEC(ZSTD(6));

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_logs_raw
TO telemetry.logs_raw
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

  time_unix_nano, observed_time_unix_nano,
  severity_number, severity_text, body_str, body_json,
  trace_id_bin, span_id_bin,
  toString(attr_v_str) AS attributes_json,
  ingested_at
FROM telemetry.logs_stage;
