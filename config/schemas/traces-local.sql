-- Simplified ClickHouse table schema for OpenTelemetry spans
CREATE TABLE IF NOT EXISTS spans (
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
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time_date)
ORDER BY (service_name, operation_name, start_time, trace_id, span_id)
TTL start_time_date + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;