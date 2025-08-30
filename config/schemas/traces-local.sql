-- Raw ingestion table for minimal processing
CREATE TABLE IF NOT EXISTS spans_raw (
    -- Minimal extracted fields for partitioning/routing only
    ingested_at DateTime64(9) DEFAULT now64() CODEC(Delta, ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    trace_id String CODEC(ZSTD(3)),

    -- Raw OTLP data - all complex processing moved to ClickHouse
    raw_otlp String CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toDate(ingested_at)
ORDER BY (service_name, ingested_at, trace_id)
TTL toDate(ingested_at) + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Final processed spans table (target for materialized view)
CREATE TABLE IF NOT EXISTS spans_final (
    -- Core trace identifiers
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    parent_span_id String CODEC(ZSTD(3)),

    -- Span metadata
    operation_name LowCardinality(String),
    service_name LowCardinality(String),
    service_version LowCardinality(String),

    -- Timing information
    start_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    end_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    start_time_date DateTime MATERIALIZED toDateTime(start_time) CODEC(ZSTD(3)),
    duration_ns UInt64 CODEC(ZSTD(3)),

    -- Status information
    status_code LowCardinality(String),
    status_message String CODEC(ZSTD(3)),

    -- Attributes and metadata
    resource_attributes Map(String, String) CODEC(ZSTD(3)),
    span_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime64(9) CODEC(ZSTD(3)),
    source_ip IPv4 CODEC(ZSTD(3)),

    -- Keep raw data for debugging/reprocessing
    raw_otlp String CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time_date)
ORDER BY (service_name, operation_name, start_time, trace_id, span_id)
TTL start_time_date + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Real-time materialized view for processing spans
CREATE MATERIALIZED VIEW IF NOT EXISTS spans_processor TO spans_final AS
SELECT
    -- Extract core identifiers from JSON
    JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].traceId') as trace_id,
    JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].spanId') as span_id,
    JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].parentSpanId') as parent_span_id,

    -- Extract span metadata
    JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].name') as operation_name,
    service_name, -- Already extracted at ingestion
    JSONExtractString(raw_otlp, 'resourceSpans[0].resource.attributes[?(@.key=="service.version")].value.stringValue') as service_version,

    -- Extract timing information
    toDateTime64(JSONExtractUInt(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].startTimeUnixNano') / 1000000000, 9) as start_time,
    toDateTime64(JSONExtractUInt(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].endTimeUnixNano') / 1000000000, 9) as end_time,
    JSONExtractUInt(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].endTimeUnixNano') - JSONExtractUInt(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].startTimeUnixNano') as duration_ns,

    -- Extract status (defaulting to OK if not present)
    coalesce(JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].status.code'), 'OK') as status_code,
    coalesce(JSONExtractString(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].status.message'), '') as status_message,

    -- Extract attributes as maps (simplified - can be enhanced later)
    CAST(JSONExtract(raw_otlp, 'resourceSpans[0].resource.attributes', 'Map(String, String)'), 'Map(String, String)') as resource_attributes,
    CAST(JSONExtract(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0].attributes', 'Map(String, String)'), 'Map(String, String)') as span_attributes,

    -- Preserve ingestion metadata
    ingested_at,
    source_ip,
    raw_otlp

FROM spans_raw
WHERE JSONHas(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0]'); -- Only process valid spans

-- Create view alias for backward compatibility
-- This allows existing queries to work while we migrate
CREATE VIEW IF NOT EXISTS spans AS
SELECT * FROM spans_final;

ALTER TABLE spans_final
  -- tenancy & source
  ADD COLUMN IF NOT EXISSTS tenant LowCardinality(String) AFTER trace_id
  ADD COLUMN IF NOT EXISTS source_id LowCardinality(String) AFTER source_ip,
  ADD COLUMN IF NOT EXISTS producer_id LowCardinality(String),

  -- raw/calibrated
  ADD COLUMN IF NOT EXISTS start_time_raw DateTime64(9) DEFAULT start_time,
  ADD COLUMN IF NOT EXISTS end_time_raw DateTime64(9) DEFAULT end_time,
  ADD COLUMN IF NOT EXISTS start_time_cal DateTime64(9),
  ADD COLUMN IF NOT EXISTS end_time_cal   DateTime64(9),

  -- calibration model outputs (per span; start/end may differ slightly)
  ADD COLUMN IF NOT EXISTS start_offset_ns Int32,
  ADD COLUMN IF NOT EXISTS end_offset_ns  Int32,
  ADD COLUMN IF NOT EXISTS drift_ppm  Int32,
  ADD COLUMN IF NOT EXISTS start_u_ns UInt32, -- uncertainty half-width
  ADD COLUMN IF NOT EXISTS end_u_ns UInt32,

  -- causal total order (Hybrid Logical Clock)
  ADD COLUMN IF NOT EXISTS hlc_wall_ns Int64, -- derived from *_cal
  ADD COLUMN IF NOT EXISTS hlc_logical UInt32,

  -- indexing
  ADD COLUMN IF NOT EXISTS trace_id_hash UInt64,
  ADD COLUMN IF NOT EXISTS span_id_hash UInt64,
  ADD COLUMN IF NOT EXISTS parent_span_hash UInt64,

  -- bookkeeping
  ADD COLUMN IF NOT EXISTS calib_epoch LowCardinality(String);

ALTER TABLE spans_final
  MODIFY ORDER BY (
    tenant,
    start_time_cal, -- primary time
    hlc_wall_ns, hlc_logical,
    service_name,
    trace_id_hash, span_id_hash

);

ALTER TABLE spans_final
  ADD INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 64,
  ADD INDEX idx_span_id span_id TYPE bloom_filter(0.01) GRANULARITY 64;
ALTER TABLE spans_final
  MODIFY TTL ingested_at + INTERVAL 30 DAY DELETE WHERE raw_otlp != '';

ALTER TABLE spans_raw
  ADD COLUMN IF NOT EXISTS ingest_row_id UUID DEFAULT generateUUIDv4() AFTER raw_otlp;
