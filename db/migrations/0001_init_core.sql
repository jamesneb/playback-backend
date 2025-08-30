-- Migration 0001: Initialize core telemetry tables
-- Creates the foundational tables for OpenTelemetry data ingestion and processing

-- Raw ingestion table for minimal processing
CREATE TABLE IF NOT EXISTS ${DB}.spans_raw (
    -- Minimal extracted fields for partitioning/routing only
    ingested_at DateTime64(9) DEFAULT now64() CODEC(Delta, ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    trace_id String CODEC(ZSTD(3)),

    -- Raw OTLP data - all complex processing moved to ClickHouse
    raw_otlp String CODEC(ZSTD(3)),
    
    -- Unique identifier for each ingested row
    ingest_row_id UUID DEFAULT generateUUIDv4() CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toDate(ingested_at)
ORDER BY (service_name, ingested_at, trace_id)
TTL toDate(ingested_at) + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Final processed spans table with full schema
CREATE TABLE IF NOT EXISTS ${DB}.spans_final (
    -- Core identifiers
    trace_id String CODEC(ZSTD(3)),
    tenant LowCardinality(String) DEFAULT 'default' CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    parent_span_id String CODEC(ZSTD(3)),

    -- Span metadata
    operation_name LowCardinality(String) CODEC(ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    service_version LowCardinality(String) CODEC(ZSTD(3)),

    -- Timing information (raw timestamps)
    start_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    end_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    start_time_date DateTime MATERIALIZED toDateTime(start_time) CODEC(ZSTD(3)),
    duration_ns UInt64 CODEC(ZSTD(3)),

    -- Status information
    status_code LowCardinality(String) CODEC(ZSTD(3)),
    status_message String CODEC(ZSTD(3)),

    -- Attributes and metadata
    resource_attributes Map(String, String) CODEC(ZSTD(3)),
    span_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime64(9) CODEC(ZSTD(3)),
    source_ip IPv4 CODEC(ZSTD(3)),
    source_id LowCardinality(String) DEFAULT 'unknown' CODEC(ZSTD(3)),
    producer_id LowCardinality(String) DEFAULT 'unknown' CODEC(ZSTD(3)),

    -- Raw/calibrated timestamps
    start_time_raw DateTime64(9) DEFAULT start_time CODEC(Delta, ZSTD(3)),
    end_time_raw DateTime64(9) DEFAULT end_time CODEC(Delta, ZSTD(3)),
    start_time_cal DateTime64(9) DEFAULT start_time CODEC(Delta, ZSTD(3)),
    end_time_cal DateTime64(9) DEFAULT end_time CODEC(Delta, ZSTD(3)),

    -- Calibration model outputs
    start_offset_ns Int32 DEFAULT 0 CODEC(ZSTD(3)),
    end_offset_ns Int32 DEFAULT 0 CODEC(ZSTD(3)),
    drift_ppm Int32 DEFAULT 0 CODEC(ZSTD(3)),
    start_u_ns UInt32 DEFAULT 0 CODEC(ZSTD(3)),
    end_u_ns UInt32 DEFAULT 0 CODEC(ZSTD(3)),

    -- Causal total order (Hybrid Logical Clock)
    hlc_wall_ns Int64 DEFAULT toUnixTimestamp64Nano(start_time_cal) CODEC(ZSTD(3)),
    hlc_logical UInt32 DEFAULT 0 CODEC(ZSTD(3)),

    -- Indexing
    trace_id_hash UInt64 DEFAULT cityHash64(trace_id) CODEC(ZSTD(3)),
    span_id_hash UInt64 DEFAULT cityHash64(span_id) CODEC(ZSTD(3)),
    parent_span_hash UInt64 DEFAULT cityHash64(parent_span_id) CODEC(ZSTD(3)),

    -- Bookkeeping
    calib_epoch LowCardinality(String) DEFAULT 'v1' CODEC(ZSTD(3)),

    -- Keep raw data for debugging/reprocessing
    raw_otlp String CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time_cal)
ORDER BY (tenant, start_time_cal, hlc_wall_ns, hlc_logical, service_name, trace_id_hash, span_id_hash)
TTL toDateTime(ingested_at) + INTERVAL 30 DAY DELETE WHERE raw_otlp != ''
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Real-time materialized view for processing spans
CREATE MATERIALIZED VIEW IF NOT EXISTS ${DB}.spans_processor TO ${DB}.spans_final AS
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

FROM ${DB}.spans_raw
WHERE JSONHas(raw_otlp, 'resourceSpans[0].scopeSpans[0].spans[0]'); -- Only process valid spans

-- Create view alias for backward compatibility
CREATE VIEW IF NOT EXISTS ${DB}.spans AS
SELECT * FROM ${DB}.spans_final;

-- Basic metrics table
CREATE TABLE IF NOT EXISTS ${DB}.metrics (
    -- Core identifiers
    metric_name LowCardinality(String),
    service_name LowCardinality(String),
    
    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    
    -- Value (simplified - supports gauge/counter patterns)
    value Float64,
    
    -- Labels/attributes
    labels Map(String, String) CODEC(ZSTD(3)),
    
    -- Metadata
    ingested_at DateTime64(9) DEFAULT now64() CODEC(Delta, ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))
    
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp)
TTL toDateTime(timestamp) + INTERVAL 90 DAY DELETE
SETTINGS index_granularity = 8192;

-- Basic logs table
CREATE TABLE IF NOT EXISTS ${DB}.logs (
    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    observed_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    
    -- Core content
    severity_text LowCardinality(String),
    severity_number UInt8,
    body String CODEC(ZSTD(3)),
    
    -- Context
    service_name LowCardinality(String),
    trace_id String,
    span_id String,
    
    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    
    -- Metadata
    ingested_at DateTime64(9) DEFAULT now64() CODEC(Delta, ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))
    
) ENGINE = MergeTree()
PARTITION BY toDate(timestamp)
ORDER BY (service_name, timestamp, severity_number)
TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
SETTINGS index_granularity = 8192;