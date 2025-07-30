-- Simplified ClickHouse table schema for OpenTelemetry logs
CREATE TABLE IF NOT EXISTS logs (
    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    observed_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    timestamp_date DateTime MATERIALIZED toDateTime(timestamp) CODEC(ZSTD(3)),

    -- Trace correlation
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    trace_flags UInt8 CODEC(ZSTD(3)),

    -- Log content
    severity_number UInt8 CODEC(ZSTD(3)),
    severity_text LowCardinality(String),
    body String CODEC(ZSTD(3)),
    
    -- Service information
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    
    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp_date)
ORDER BY (service_name, severity_number, timestamp, trace_id)
TTL timestamp_date + INTERVAL 30 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;