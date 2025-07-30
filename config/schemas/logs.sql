-- ClickHouse table schema for OpenTelemetry logs
-- This file defines the structure for storing structured log data

-- Main logs table
CREATE TABLE IF NOT EXISTS logs (
    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    observed_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Trace correlation
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    trace_flags UInt8 CODEC(ZSTD(3)),

    -- Severity
    severity_text LowCardinality(String),
    severity_number Enum8(
        'SEVERITY_NUMBER_UNSPECIFIED' = 0,
        'TRACE' = 1,
        'TRACE2' = 2,
        'TRACE3' = 3,
        'TRACE4' = 4,
        'DEBUG' = 5,
        'DEBUG2' = 6,
        'DEBUG3' = 7,
        'DEBUG4' = 8,
        'INFO' = 9,
        'INFO2' = 10,
        'INFO3' = 11,
        'INFO4' = 12,
        'WARN' = 13,
        'WARN2' = 14,
        'WARN3' = 15,
        'WARN4' = 16,
        'ERROR' = 17,
        'ERROR2' = 18,
        'ERROR3' = 19,
        'ERROR4' = 20,
        'FATAL' = 21,
        'FATAL2' = 22,
        'FATAL3' = 23,
        'FATAL4' = 24
    ),

    -- Service information
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String CODEC(ZSTD(3)),

    -- Log content
    body String CODEC(ZSTD(3)),

    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, severity_number, timestamp, trace_id)
TTL timestamp + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;

-- Materialized view for log level statistics
CREATE MATERIALIZED VIEW IF NOT EXISTS log_stats_mv
TO log_stats
AS SELECT
    service_name,
    severity_number,
    severity_text,
    toStartOfMinute(timestamp) as timestamp,
    count() as log_count,
    uniq(trace_id) as unique_traces,
    countIf(trace_id != '') as traced_logs,
    length(groupArray(body)[1]) as avg_body_length
FROM logs
GROUP BY service_name, severity_number, severity_text, timestamp;

-- Supporting table for log statistics
CREATE TABLE IF NOT EXISTS log_stats (
    service_name LowCardinality(String),
    severity_number Enum8(
        'SEVERITY_NUMBER_UNSPECIFIED' = 0,
        'TRACE' = 1,
        'TRACE2' = 2,
        'TRACE3' = 3,
        'TRACE4' = 4,
        'DEBUG' = 5,
        'DEBUG2' = 6,
        'DEBUG3' = 7,
        'DEBUG4' = 8,
        'INFO' = 9,
        'INFO2' = 10,
        'INFO3' = 11,
        'INFO4' = 12,
        'WARN' = 13,
        'WARN2' = 14,
        'WARN3' = 15,
        'WARN4' = 16,
        'ERROR' = 17,
        'ERROR2' = 18,
        'ERROR3' = 19,
        'ERROR4' = 20,
        'FATAL' = 21,
        'FATAL2' = 22,
        'FATAL3' = 23,
        'FATAL4' = 24
    ),
    severity_text LowCardinality(String),
    timestamp DateTime,
    log_count UInt64,
    unique_traces UInt64,
    traced_logs UInt64,
    avg_body_length UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, severity_number, timestamp)
TTL timestamp + INTERVAL 90 DAY DELETE;

-- Error logs table for fast error querying
CREATE MATERIALIZED VIEW IF NOT EXISTS error_logs_mv
TO error_logs
AS SELECT
    timestamp,
    trace_id,
    span_id,
    service_name,
    service_version,
    service_instance_id,
    severity_text,
    severity_number,
    body,
    attributes,
    resource_attributes,
    ingested_at
FROM logs
WHERE severity_number >= 17;  -- ERROR and above

-- Supporting table for error logs
CREATE TABLE IF NOT EXISTS error_logs (
    timestamp DateTime64(9),
    trace_id String,
    span_id String,
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String,
    severity_text LowCardinality(String),
    severity_number UInt8,
    body String,
    attributes Map(String, String),
    resource_attributes Map(String, String),
    ingested_at DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, timestamp, trace_id)
TTL timestamp + INTERVAL 90 DAY DELETE;

-- Full-text search table for log content
CREATE TABLE IF NOT EXISTS logs_search (
    timestamp DateTime64(9),
    service_name LowCardinality(String),
    trace_id String,
    body String,
    attributes_text String,  -- Flattened attributes for search
    search_vector String     -- Preprocessed text for full-text search
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, timestamp)
TTL timestamp + INTERVAL 30 DAY DELETE;

-- Materialized view to populate search table
CREATE MATERIALIZED VIEW IF NOT EXISTS logs_search_mv
TO logs_search
AS SELECT
    timestamp,
    service_name,
    trace_id,
    body,
    arrayStringConcat(arrayConcat(
        mapKeys(attributes),
        mapValues(attributes)
    ), ' ') as attributes_text,
    concat(
        body, ' ',
        arrayStringConcat(arrayConcat(
            mapKeys(attributes),
            mapValues(attributes)
        ), ' ')
    ) as search_vector
FROM logs;

-- Indexes for performance
-- Full-text search index on body
 ALTER TABLE logs ADD INDEX idx_body_fulltext body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1;

-- Index for trace correlation
 ALTER TABLE logs ADD INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 1;

-- Index for service filtering
 ALTER TABLE logs ADD INDEX idx_service_name service_name TYPE set(100) GRANULARITY 1;

-- Index for severity filtering
 ALTER TABLE logs ADD INDEX idx_severity severity_number TYPE set(0) GRANULARITY 1;

-- Index for attributes
 ALTER TABLE logs ADD INDEX idx_attributes mapKeys(attributes) TYPE bloom_filter GRANULARITY 1;
