# spans_final

The main processed spans table containing full OpenTelemetry trace data with calibration support.

## Purpose

- Store processed trace spans with full schema
- Support multi-tenant data isolation
- Provide clock-calibrated timestamps for accurate ordering
- Enable fast time-range and trace-based queries

## Schema

| Column | Type | Description |
|--------|------|-------------|
| `trace_id` | String | OpenTelemetry trace identifier |
| `tenant` | LowCardinality(String) | Multi-tenant isolation key (default: 'default') |
| `span_id` | String | Unique span identifier within trace |
| `parent_span_id` | String | Parent span ID for building trace tree |
| `operation_name` | LowCardinality(String) | Span operation name |
| `service_name` | LowCardinality(String) | Service that generated the span |
| `service_version` | LowCardinality(String) | Service version |
| `start_time` | DateTime64(9) | Raw start timestamp from source |
| `end_time` | DateTime64(9) | Raw end timestamp from source |
| `start_time_cal` | DateTime64(9) | Clock-calibrated start timestamp |
| `end_time_cal` | DateTime64(9) | Clock-calibrated end timestamp |
| `duration_ns` | UInt64 | Span duration in nanoseconds |
| `status_code` | LowCardinality(String) | Span status (OK, ERROR, etc.) |
| `status_message` | String | Optional status message |
| `hlc_wall_ns` | Int64 | Hybrid Logical Clock wall time |
| `hlc_logical` | UInt32 | Hybrid Logical Clock logical counter |
| `trace_id_hash` | UInt64 | Hash of trace_id for indexing |
| `span_id_hash` | UInt64 | Hash of span_id for indexing |
| `resource_attributes` | Map(String, String) | OTLP resource attributes |
| `span_attributes` | Map(String, String) | OTLP span attributes |
| `source_id` | LowCardinality(String) | Source system identifier |
| `producer_id` | LowCardinality(String) | Data producer identifier |
| `calib_epoch` | LowCardinality(String) | Calibration model version |
| `ingested_at` | DateTime64(9) | When span was ingested |
| `raw_otlp` | String | Original OTLP JSON for debugging |

## Indexes

- **Primary Key**: `(tenant, start_time_cal, hlc_wall_ns, hlc_logical, service_name, trace_id_hash, span_id_hash)`
- **Bloom Filter**: `trace_id`, `span_id` for fast lookups
- **Partition**: By `toYYYYMM(start_time_cal)` for efficient time-range queries

## Common Queries

### Get Trace by ID
```sql
SELECT * FROM spans_final 
WHERE tenant = 'default' 
  AND trace_id = 'your-trace-id'
ORDER BY start_time_cal;
```

### Service Performance (P95 Duration)
```sql
SELECT service_name, 
       quantile(0.95)(duration_ns / 1000000) as p95_duration_ms
FROM spans_final
WHERE tenant = 'default'
  AND start_time_cal >= now() - INTERVAL 1 HOUR
GROUP BY service_name
ORDER BY p95_duration_ms DESC;
```

### Error Rate by Service
```sql
SELECT service_name,
       countIf(status_code = 'ERROR') * 100.0 / count() as error_rate
FROM spans_final  
WHERE tenant = 'default'
  AND start_time_cal >= now() - INTERVAL 1 HOUR
GROUP BY service_name
HAVING count() > 100
ORDER BY error_rate DESC;
```

## Retention

- **TTL**: 30 days after ingestion
- **Partitioning**: Monthly partitions for efficient data management
- **Raw Data**: `raw_otlp` deleted after TTL for space efficiency

## Performance Notes

- Optimized for time-range queries using `start_time_cal`
- Hash indexes enable O(1) trace/span lookups
- Compression reduces storage by ~70%
- Materialized views provide real-time processing