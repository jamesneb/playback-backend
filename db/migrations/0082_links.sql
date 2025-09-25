-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

-- Target "exploded" links table
CREATE TABLE IF NOT EXISTS telemetry.span_links_raw
(
  tenant_id String,

  -- source (owner) span
  trace_id_bin FixedString(16),
  span_id_bin  FixedString(8),
  trace_id_hex String ALIAS lower(hex(trace_id_bin)),
  span_id_hex  String ALIAS lower(hex(span_id_bin)),

  -- linked span
  linked_trace_id_bin FixedString(16),
  linked_span_id_bin  FixedString(8),
  linked_trace_id_hex String ALIAS lower(hex(linked_trace_id_bin)),
  linked_span_id_hex  String ALIAS lower(hex(linked_span_id_bin)),

  attributes_json Object('json') DEFAULT CAST('{}','Object(\'json\')'),
  ingested_at DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDate(ingested_at)
ORDER BY (tenant_id, trace_id_bin, span_id_bin, linked_trace_id_bin, linked_span_id_bin)
SETTINGS index_granularity = 8192;

-- MV: stage → links
CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_span_links
TO telemetry.span_links_raw
AS
SELECT
  tenant_id,
  trace_id_bin,
  span_id_bin,
  l_trace_id  AS linked_trace_id_bin,
  l_span_id   AS linked_span_id_bin,
  toString(l_attr_v_str) AS attributes_json,
  ingested_at
FROM telemetry.spans_stage
ARRAY JOIN
  link_trace_id_bin AS l_trace_id,
  link_span_id_bin  AS l_span_id,
  link_attr_key     AS l_attr_key,
  link_attr_v_str   AS l_attr_v_str;

