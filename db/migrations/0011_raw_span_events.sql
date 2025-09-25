-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

CREATE TABLE IF NOT EXISTS telemetry.span_events_raw
(
  tenant_id String,
  trace_id_bin FixedString(16),
  span_id_bin  FixedString(8),
  trace_id_hex String ALIAS lower(hex(trace_id_bin)),
  span_id_hex  String ALIAS lower(hex(span_id_bin)),

  time_unix_nano UInt64,
  name String,
  attributes_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),
  ingested_at DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(time_unix_nano))
ORDER BY (tenant_id, time_unix_nano, trace_id_bin, span_id_bin);

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_span_events
TO telemetry.span_events_raw
AS
SELECT
  tenant_id,
  trace_id_bin, span_id_bin,
  ev_time       AS time_unix_nano,
  ev_name       AS name,
  toString(ev_attr_v_str) AS attributes_json,
  ingested_at
FROM telemetry.spans_stage
ARRAY JOIN
  event_time_unix_nano AS ev_time,
  event_name           AS ev_name,
  event_attr_key       AS ev_attr_key,
  event_attr_v_str     AS ev_attr_v_str;
