-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.spans_raw
(
  -- tenancy & identity
  tenant_id            String,
  service_namespace    LowCardinality(String) DEFAULT '',
  service_name         LowCardinality(String) DEFAULT '',
  service_version      LowCardinality(String) DEFAULT '',
  service_instance_id  String                 DEFAULT '',
  host_id              LowCardinality(String) DEFAULT '',

  -- ids (binary + hex aliases)
  trace_id_bin       FixedString(16),
  span_id_bin        FixedString(8),
  parent_span_id_bin FixedString(8),
  trace_id_hex       String ALIAS lower(hex(trace_id_bin)),
  span_id_hex        String ALIAS lower(hex(span_id_bin)),
  parent_span_id_hex String ALIAS lower(hex(parent_span_id_bin)),

  -- span basics
  name                 String,
  kind                 Enum8('UNSPECIFIED'=0, 'INTERNAL'=1,'SERVER'=2,'CLIENT'=3,'PRODUCER'=4,'CONSUMER'=5) DEFAULT 'UNSPECIFIED',
  start_time_unix_nano UInt64,
  end_time_unix_nano   UInt64,

  -- aliases for convenience
  start_ns             UInt64 ALIAS start_time_unix_nano,
  end_ns               UInt64 ALIAS end_time_unix_nano,

  status_code          UInt8  DEFAULT 0,
  status_message       String DEFAULT '',

  ingested_at          DateTime64(9) DEFAULT now64(),
  ingest_ns            UInt64 MATERIALIZED toUnixTimestamp64Nano(ingested_at),

  resource_json        Object('json') DEFAULT CAST('{}','Object(\'json\')'),
  attributes_json      Object('json') DEFAULT CAST('{}','Object(\'json\')')
)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(start_time_unix_nano))
ORDER BY (tenant_id, service_name,host_id, start_time_unix_nano, trace_id_bin)
SETTINGS index_granularity = 8192;

-- Drop indexes if they exist, then recreate them to be idempotent
ALTER TABLE telemetry.spans_raw DROP INDEX IF EXISTS idx_trace;
ALTER TABLE telemetry.spans_raw DROP INDEX IF EXISTS idx_parent;
ALTER TABLE telemetry.spans_raw ADD INDEX idx_trace trace_id_bin TYPE bloom_filter GRANULARITY 64;
ALTER TABLE telemetry.spans_raw ADD INDEX idx_parent parent_span_id_bin TYPE bloom_filter GRANULARITY 64;

-- Space savers
ALTER TABLE telemetry.spans_raw
  MODIFY COLUMN name CODEC(ZSTD(6)),
  MODIFY COLUMN status_message CODEC(ZSTD(6)),
  MODIFY COLUMN resource_json CODEC(ZSTD(6)),
  MODIFY COLUMN attributes_json CODEC(ZSTD(6));

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_spans_raw
TO telemetry.spans_raw
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

  trace_id_bin, span_id_bin, parent_span_id_bin,

  name,
  kind,
  start_time_unix_nano, end_time_unix_nano,
  status_code, status_message, ingested_at,

  -- lightweight JSON mirrors - using toString for compatibility
  toString(resource_attr_v_str) AS resource_json,
  toString(attr_v_str) AS attributes_json
FROM telemetry.spans_stage;
