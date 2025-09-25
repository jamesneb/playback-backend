CREATE TABLE IF NOT EXISTS telemetry.spans_stage
(
  tenant_id String DEFAULT '',

  /* Resource attributes */
  resource_attr_key     Array(String),
  resource_attr_v_str   Array(String),
  resource_attr_v_i64   Array(Int64),
  resource_attr_v_f64   Array(Float64),
  resource_attr_v_bool  Array(UInt8),
  resource_attr_v_bytes Array(String),

  /* Instrumentation Scope */
  scope_name String DEFAULT '',
  scope_version String DEFAULT '',

  /* Core  IDs (binary) + hex ALIASES */
  trace_id_bin        FixedString(16),
  span_id_bin         FixedString(8),
  parent_span_id_bin  FixedString(8),
  trace_id_hex        String ALIAS lower(hex(trace_id_bin)),
  span_id_hex         String ALIAS lower(hex(span_id_bin)),
  parent_span_id_hex  String ALIAS lower(hex(parent_span_id_bin)),

  /* Span fields */
  name                  String,
  kind                  UInt8,   -- maps to telemetry.SpanKind in MV
  start_time_unix_nano  UInt64,
  end_time_unix_nano    UInt64,

  /* Span attributes (typed,, parallel arrays) */
 attr_key     Array(String),
 attr_v_str   Array(String),
 attr_v_i64   Array(Int64),
 attr_v_f64   Array(Float64),
 attr_v_bool  Array(UInt8),
 attr_v_bytes Array(String),

/* Status */
status_code     UInt8 DEFAULT 0,
status_message  String DEFAULT '',

/* Events */
event_time_unix_nano Array(UInt64),
event_name           Array(String),
event_attr_key       Array(Array(String)),
event_attr_v_str     Array(Array(String)),
event_attr_v_i64     Array(Array(Int64)),
event_attr_v_f64     Array(Array(Float64)),
event_attr_v_bool    Array(Array(UInt8)),
event_attr_v_bytes   Array(Array(String)),

/* Links */
link_trace_id_bin  Array(FixedString(16)),
link_span_id_bin   Array(FixedString(8)),
link_attr_key      Array(Array(String)),
link_attr_v_str    Array(Array(String)),
link_attr_v_i64    Array(Array(Int64)),
link_attr_v_f64    Array(Array(Float64)),
link_attr_v_bool   Array(Array(UInt8)),
link_attr_v_bytes  Array(Array(String)),

/* Ingest timestamp */
ingested_at DateTime64(9) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toDateNs(toInt64(start_time_unix_nano))
ORDER BY (tenant_id, start_time_unix_nano, trace_id_bin)
SETTINGS index_granularity = 8192;

-- Compression
ALTER TABLE telemetry.spans_stage
  MODIFY COLUMN name CODEC(ZSTD(6)),
  MODIFY COLUMN status_message CODEC(ZSTD(6));
