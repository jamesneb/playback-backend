-- ClickHouse initialization script for telemetry data
-- This script creates the necessary databases and tables for storing OpenTelemetry data

-- Create the main telemetry database
CREATE DATABASE IF NOT EXISTS telemetry;

-- Use the telemetry database
USE telemetry;

-- =============================================================================
-- TRACES TABLES
-- =============================================================================

-- Main traces table for storing distributed traces
CREATE TABLE IF NOT EXISTS traces (
    trace_id String,
    span_id String,
    parent_span_id String,
    operation_name String,
    service_name String,
    service_version String,
    start_time DateTime64(9),
    end_time DateTime64(9),
    duration_ns UInt64,
    status_code Int8,
    status_message String,
    span_kind Int8,
    resource_attributes Map(String, String),
    span_attributes Map(String, String),
    events Array(Tuple(timestamp DateTime64(9), name String, attributes Map(String, String))),
    links Array(Tuple(trace_id String, span_id String, attributes Map(String, String))),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time)
ORDER BY (service_name, start_time, trace_id, span_id)
TTL start_time + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192;

-- Materialized view for trace metrics aggregation
CREATE MATERIALIZED VIEW IF NOT EXISTS trace_metrics_mv
TO trace_metrics
AS SELECT
    service_name,
    operation_name,
    status_code,
    toStartOfMinute(start_time) as minute,
    count() as span_count,
    avg(duration_ns) as avg_duration_ns,
    quantile(0.95)(duration_ns) as p95_duration_ns,
    quantile(0.99)(duration_ns) as p99_duration_ns,
    sum(case when status_code != 0 then 1 else 0 end) as error_count
FROM traces
GROUP BY service_name, operation_name, status_code, minute;

CREATE TABLE IF NOT EXISTS trace_metrics (
    service_name String,
    operation_name String,
    status_code Int8,
    minute DateTime,
    span_count UInt64,
    avg_duration_ns Float64,
    p95_duration_ns UInt64,
    p99_duration_ns UInt64,
    error_count UInt64
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (service_name, operation_name, minute)
TTL minute + INTERVAL 30 DAY DELETE;

-- =============================================================================
-- METRICS TABLES
-- =============================================================================

-- Time series metrics table
CREATE TABLE IF NOT EXISTS metrics (
    metric_name String,
    metric_type Enum8('counter' = 1, 'gauge' = 2, 'histogram' = 3, 'summary' = 4),
    service_name String,
    service_version String,
    timestamp DateTime64(9),
    value Float64,
    labels Map(String, String),
    resource_attributes Map(String, String),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp)
TTL timestamp + INTERVAL 30 DAY DELETE
SETTINGS index_granularity = 8192;

-- Histogram buckets table for detailed histogram data
CREATE TABLE IF NOT EXISTS metric_histograms (
    metric_name String,
    service_name String,
    timestamp DateTime64(9),
    bucket_le Float64,
    bucket_count UInt64,
    labels Map(String, String),
    resource_attributes Map(String, String),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, metric_name, timestamp, bucket_le)
TTL timestamp + INTERVAL 30 DAY DELETE;

-- =============================================================================
-- LOGS TABLES
-- =============================================================================

-- Main logs table
CREATE TABLE IF NOT EXISTS logs (
    timestamp DateTime64(9),
    trace_id String,
    span_id String,
    severity_text String,
    severity_number Int8,
    service_name String,
    service_version String,
    body String,
    attributes Map(String, String),
    resource_attributes Map(String, String),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (service_name, timestamp, trace_id)
TTL timestamp + INTERVAL 7 DAY DELETE
SETTINGS index_granularity = 8192;

-- =============================================================================
-- SERVICE TOPOLOGY TABLES
-- =============================================================================

-- Service dependencies derived from traces
CREATE TABLE IF NOT EXISTS service_dependencies (
    parent_service String,
    child_service String,
    operation_name String,
    timestamp DateTime,
    call_count UInt64,
    avg_duration_ns Float64,
    error_rate Float64
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (parent_service, child_service, operation_name, timestamp);

-- Materialized view to populate service dependencies from traces
CREATE MATERIALIZED VIEW IF NOT EXISTS service_dependencies_mv
TO service_dependencies
AS SELECT
    parent.service_name as parent_service,
    child.service_name as child_service,
    child.operation_name as operation_name,
    toStartOfHour(child.start_time) as timestamp,
    count() as call_count,
    avg(child.duration_ns) as avg_duration_ns,
    countIf(child.status_code != 0) / count() as error_rate
FROM traces child
LEFT JOIN traces parent ON child.parent_span_id = parent.span_id AND child.trace_id = parent.trace_id
WHERE parent.service_name != child.service_name AND parent.service_name != ''
GROUP BY parent_service, child_service, operation_name, timestamp;

-- =============================================================================
-- SYSTEM MONITORING TABLES
-- =============================================================================

-- Table for storing system health metrics
CREATE TABLE IF NOT EXISTS system_health (
    timestamp DateTime,
    component String,
    metric_name String,
    value Float64,
    status Enum8('healthy' = 1, 'warning' = 2, 'critical' = 3),
    details String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (component, timestamp)
TTL timestamp + INTERVAL 30 DAY DELETE;

-- =============================================================================
-- INDEXES FOR PERFORMANCE
-- =============================================================================

-- Create additional indexes for common query patterns
-- These will be created as the system learns about query patterns

-- =============================================================================
-- INSERT SAMPLE DATA FOR TESTING
-- =============================================================================

-- Insert sample trace data
INSERT INTO traces VALUES
(
    '00000000000000000000000000000001',
    '0000000000000001',
    '',
    'POST /orders',
    'order-service',
    '1.0.0',
    '2024-01-01 12:00:00.000',
    '2024-01-01 12:00:01.234',
    1234000000,
    0,
    '',
    1,
    {'deployment.environment': 'local', 'host.name': 'localhost'},
    {'http.method': 'POST', 'http.route': '/orders', 'http.status_code': '201'},
    [('2024-01-01 12:00:00.500', 'order.validated', {'validation.result': 'success'})],
    [],
    now()
);

-- Insert sample metrics data
INSERT INTO metrics VALUES
(
    'orders_total',
    'counter',
    'order-service',
    '1.0.0',
    '2024-01-01 12:00:00.000',
    1.0,
    {'status': 'success'},
    {'deployment.environment': 'local'},
    now()
);

-- Insert sample log data
INSERT INTO logs VALUES
(
    '2024-01-01 12:00:00.000',
    '00000000000000000000000000000001',
    '0000000000000001',
    'INFO',
    9,
    'order-service',
    '1.0.0',
    'Order created successfully',
    {'order.id': 'order_123', 'user.id': 'user_456'},
    {'deployment.environment': 'local'},
    now()
);

-- =============================================================================
-- UTILITY FUNCTIONS AND VIEWS
-- =============================================================================

-- View for service overview
CREATE VIEW IF NOT EXISTS service_overview AS
SELECT
    service_name,
    service_version,
    count() as total_spans,
    uniq(trace_id) as total_traces,
    avg(duration_ns) as avg_duration_ns,
    countIf(status_code != 0) / count() * 100 as error_rate_percent,
    min(start_time) as first_seen,
    max(start_time) as last_seen
FROM traces
WHERE start_time >= now() - INTERVAL 1 DAY
GROUP BY service_name, service_version
ORDER BY total_spans DESC;

-- View for recent errors
CREATE VIEW IF NOT EXISTS recent_errors AS
SELECT
    service_name,
    operation_name,
    trace_id,
    span_id,
    start_time,
    status_message,
    span_attributes
FROM traces
WHERE status_code != 0
  AND start_time >= now() - INTERVAL 1 HOUR
ORDER BY start_time DESC
LIMIT 100;

-- View for slowest operations
CREATE VIEW IF NOT EXISTS slowest_operations AS
SELECT
    service_name,
    operation_name,
    avg(duration_ns) as avg_duration_ns,
    quantile(0.95)(duration_ns) as p95_duration_ns,
    count() as call_count
FROM traces
WHERE start_time >= now() - INTERVAL 1 HOUR
GROUP BY service_name, operation_name
HAVING count() >= 10
ORDER BY avg_duration_ns DESC
LIMIT 50;