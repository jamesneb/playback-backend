-- Simplified ClickHouse table schema for OpenTelemetry metrics
CREATE TABLE IF NOT EXISTS metrics (
    -- Core identifiers
    metric_name LowCardinality(String),
    service_name LowCardinality(String),
    
    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    timestamp_date DateTime MATERIALIZED toDateTime(timestamp) CODEC(ZSTD(3)),
    
    -- Metric data
    metric_type LowCardinality(String),
    value Float64 CODEC(ZSTD(3)),
    
    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),
    
    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3)),
    source_ip IPv4 DEFAULT toIPv4('0.0.0.0') CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp_date)
ORDER BY (service_name, metric_name, timestamp)
TTL timestamp_date + INTERVAL 90 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1,
    merge_with_ttl_timeout = 3600;