-- ClickHouse table schema for OpenTelemetry metrics
-- This file defines the structure for storing time-series metrics data

-- Main metrics table for all metric types
CREATE TABLE IF NOT EXISTS metrics (
    -- Metric identification
    metric_name LowCardinality(String),
    metric_type Enum8(
        'METRIC_TYPE_UNSPECIFIED' = 0,
        'GAUGE' = 1,
        'COUNTER' = 2,
        'HISTOGRAM' = 3,
        'EXPONENTIAL_HISTOGRAM' = 4,
        'SUMMARY' = 5
    ),

    -- Service information
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String CODEC(ZSTD(3)),

    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Value (for gauges and counters)
    value Float64 CODEC(ZSTD(3)),

    -- Labels/dimensions
    labels Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Metric metadata
    unit String CODEC(ZSTD(3)),
    description String CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp)
TTL timestamp + INTERVAL 90 DAY DELETE
SETTINGS
    index_granularity = 8192,
    ttl_only_drop_parts = 1;

-- Histogram buckets table for detailed histogram data
CREATE TABLE IF NOT EXISTS metric_histograms (
    -- Metric identification
    metric_name LowCardinality(String),
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String CODEC(ZSTD(3)),

    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Histogram data
    bucket_le Float64,  -- Upper bound of the bucket (less than or equal)
    bucket_count UInt64,

    -- Additional histogram metrics
    sum Float64,
    count UInt64,

    -- Labels/dimensions
    labels Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp, bucket_le)
TTL timestamp + INTERVAL 90 DAY DELETE
SETTINGS index_granularity = 8192;

-- Exponential histogram table for exponential histogram data
CREATE TABLE IF NOT EXISTS metric_exponential_histograms (
    -- Metric identification
    metric_name LowCardinality(String),
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String CODEC(ZSTD(3)),

    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Exponential histogram data
    count UInt64,
    sum Float64,
    scale Int32,
    zero_count UInt64,
    positive_offset Int32,
    positive_bucket_counts Array(UInt64),
    negative_offset Int32,
    negative_bucket_counts Array(UInt64),

    -- Labels/dimensions
    labels Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp)
TTL timestamp + INTERVAL 90 DAY DELETE;

-- Summary metrics table for summary data
CREATE TABLE IF NOT EXISTS metric_summaries (
    -- Metric identification
    metric_name LowCardinality(String),
    service_name LowCardinality(String),
    service_version LowCardinality(String),
    service_instance_id String CODEC(ZSTD(3)),

    -- Timing
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Summary data
    count UInt64,
    sum Float64,
    quantiles Array(Tuple(Float64, Float64)),  -- (quantile, value) pairs

    -- Labels/dimensions
    labels Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Ingestion metadata
    ingested_at DateTime DEFAULT now() CODEC(ZSTD(3))

) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp)
TTL timestamp + INTERVAL 90 DAY DELETE;

-- Materialized view for metric aggregations (1-minute resolution)
CREATE MATERIALIZED VIEW IF NOT EXISTS metrics_1min_mv
TO metrics_1min
AS SELECT
    service_name,
    metric_name,
    metric_type,
    toStartOfMinute(timestamp) as timestamp,
    avg(value) as avg_value,
    min(value) as min_value,
    max(value) as max_value,
    sum(value) as sum_value,
    count() as sample_count,
    uniq(service_instance_id) as instance_count
FROM metrics
WHERE metric_type IN ('GAUGE', 'COUNTER')
GROUP BY service_name, metric_name, metric_type, timestamp;

-- Supporting table for 1-minute aggregations
CREATE TABLE IF NOT EXISTS metrics_1min (
    service_name LowCardinality(String),
    metric_name LowCardinality(String),
    metric_type Enum8(
        'METRIC_TYPE_UNSPECIFIED' = 0,
        'GAUGE' = 1,
        'COUNTER' = 2,
        'HISTOGRAM' = 3,
        'EXPONENTIAL_HISTOGRAM' = 4,
        'SUMMARY' = 5
    ),
    timestamp DateTime,
    avg_value Float64,
    min_value Float64,
    max_value Float64,
    sum_value Float64,
    sample_count UInt64,
    instance_count UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, metric_type, timestamp)
TTL timestamp + INTERVAL 365 DAY DELETE;

-- Materialized view for hourly aggregations
CREATE MATERIALIZED VIEW IF NOT EXISTS metrics_1hour_mv
TO metrics_1hour
AS SELECT
    service_name,
    metric_name,
    metric_type,
    toStartOfHour(timestamp) as timestamp,
    avg(avg_value) as avg_value,
    min(min_value) as min_value,
    max(max_value) as max_value,
    sum(sum_value) as sum_value,
    sum(sample_count) as sample_count,
    max(instance_count) as max_instance_count
FROM metrics_1min
GROUP BY service_name, metric_name, metric_type, timestamp;

-- Supporting table for hourly aggregations
CREATE TABLE IF NOT EXISTS metrics_1hour (
    service_name LowCardinality(String),
    metric_name LowCardinality(String),
    metric_type Enum8(
        'METRIC_TYPE_UNSPECIFIED' = 0,
        'GAUGE' = 1,
        'COUNTER' = 2,
        'HISTOGRAM' = 3,
        'EXPONENTIAL_HISTOGRAM' = 4,
        'SUMMARY' = 5
    ),
    timestamp DateTime,
    avg_value Float64,
    min_value Float64,
    max_value Float64,
    sum_value Float64,
    sample_count UInt64,
    max_instance_count UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, metric_type, timestamp)
TTL timestamp + INTERVAL 2 YEAR DELETE;

-- Indexes for common query patterns
 ALTER TABLE metrics ADD INDEX idx_metric_name metric_name TYPE set(1000) GRANULARITY 1;
 ALTER TABLE metrics ADD INDEX idx_service_name service_name TYPE set(100) GRANULARITY 1;
 ALTER TABLE metrics ADD INDEX idx_labels mapKeys(labels) TYPE bloom_filter GRANULARITY 1;
