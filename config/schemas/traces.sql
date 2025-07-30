-- ClickHouse table schema for OpenTelemetry traces
-- This file defines the structure for storing distributed tracing data

CREATE TABLE IF NOT EXISTS traces (
    -- Core trace identifiers
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    parent_span_id String CODEC(ZSTD(3)),

    -- Span metadata
    operation_name LowCardinality(String),
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    span_kind Enum8(
        'SPAN_KIND_UNSPECIFIED' = 0,
        'SPAN_KIND_INTERNAL' = 1,
        'SPAN_KIND_SERVER' = 2,
        'SPAN_KIND_CLIENT' = 3,
        'SPAN_KIND_PRODUCER' = 4,
        'SPAN_KIND_CONSUMER' = 5
    ),

    -- Timing information
    start_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    end_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    duration_ns UInt64 CODEC(ZSTD(3)),

    -- Status information
    status_code Enum8(
        'STATUS_CODE_UNSET' = 0,
        'STATUS_CODE_OK' = 1,
        'STATUS_CODE_ERROR' = 2
    ),
    status_message String CODEC(ZSTD(3)),

    -- Attributes and metadata
    resource_attributes Map(String, String) CODEC(ZSTD(3)),
    span_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Events (structured as nested data)
    events Array(Tuple(
        timestamp DateTime64(9),
        name String,
        attributes Map(String, String)
    )) CODEC(ZSTD(3)),

    -- Links to other spans
    links Array(Tuple(
        trace_id String,
        span_id String,
        trace_state String,
        attributes Map(String, String)
    )) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time)
ORDER BY (service_name, operation_name, start_time, trace_id, span_id)
TTL start_time + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Materialized view for real-time trace metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS trace_metrics_realtime_mv
TO trace_metrics_realtime
AS SELECT
    service_name,
    operation_name,
    span_kind,
    status_code,
    toStartOfMinute(start_time) as timestamp,
    count() as span_count,
    avg(duration_ns) as avg_duration_ns,
    quantile(0.5)(duration_ns) as p50_duration_ns,
    quantile(0.95)(duration_ns) as p95_duration_ns,
    quantile(0.99)(duration_ns) as p99_duration_ns,
    max(duration_ns) as max_duration_ns,
    countIf(status_code = 'STATUS_CODE_ERROR') as error_count,
    uniq(trace_id) as unique_traces
FROM traces
GROUP BY service_name, operation_name, span_kind, status_code, timestamp;

-- Supporting table for the materialized view
CREATE TABLE IF NOT EXISTS trace_metrics_realtime (
    service_name LowCardinality(String),
    operation_name LowCardinality(String),
    span_kind Enum8(
        'SPAN_KIND_UNSPECIFIED' = 0,
        'SPAN_KIND_INTERNAL' = 1,
        'SPAN_KIND_SERVER' = 2,
        'SPAN_KIND_CLIENT' = 3,
        'SPAN_KIND_PRODUCER' = 4,
        'SPAN_KIND_CONSUMER' = 5
    ),
    status_code Enum8(
        'STATUS_CODE_UNSET' = 0,
        'STATUS_CODE_OK' = 1,
        'STATUS_CODE_ERROR' = 2
    ),
    timestamp DateTime,
    span_count UInt64,
    avg_duration_ns Float64,
    p50_duration_ns UInt64,
    p95_duration_ns UInt64,
    p99_duration_ns UInt64,
    max_duration_ns UInt64,
    error_count UInt64,
    unique_traces UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, operation_name, span_kind, status_code, timestamp)
TTL timestamp + INTERVAL 90 DAY DELETE;

-- Indexes for common query patterns
-- These will be added based on actual query patterns observed in production

-- Index for trace lookup by ID
 ALTER TABLE traces ADD INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1;

-- Index for service and operation filtering
 ALTER TABLE traces ADD INDEX idx_service_operation (service_name, operation_name) TYPE set(100) GRANULARITY 1;

-- Index for error spans
 ALTER TABLE traces ADD INDEX idx_errors status_code TYPE set(0) GRANULARITY 1;
