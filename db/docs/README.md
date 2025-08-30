# Database Schema Documentation

This directory contains documentation for all database tables in the telemetry system.

## Overview

The telemetry database stores OpenTelemetry data (traces, metrics, logs) with additional support for:
- Clock calibration and drift correction
- Multi-tenant data isolation  
- Causal ordering with Hybrid Logical Clock
- Real-time event processing via materialized views

## Tables by Category

### 📊 Core Telemetry Data
- [spans_raw](spans_raw.md) - Raw OTLP ingestion with minimal processing
- [spans_final](spans_final.md) - Processed spans with full schema and calibration 
- [metrics](metrics.md) - OpenTelemetry metrics data
- [logs](logs.md) - OpenTelemetry logs data

### ⚡ Real-time Processing
- [span_events](span_events.md) - Time-series events for span start/end times
- [spans_processor](spans_processor.md) - Materialized view processing raw spans
- [mv_span_events](mv_span_events.md) - Materialized view generating span events

### 🕐 Clock Calibration
- [calibration_models](calibration_models.md) - Clock drift models per source
- [calibration_anchors](calibration_anchors.md) - Timing constraints for calibration
- [calibration_watermarks](calibration_watermarks.md) - Processing progress tracking
- [calibrator_cursors](calibrator_cursors.md) - Ingestion cursor tracking

### 🗄️ System
- [schema_migrations](schema_migrations.md) - Migration tracking table

## Common Query Patterns

### Get Recent Traces by Service
```sql
SELECT trace_id, operation_name, start_time_cal, duration_ns
FROM spans_final 
WHERE tenant = 'default' 
  AND service_name = 'my-service'
  AND start_time_cal >= now() - INTERVAL 1 HOUR
ORDER BY start_time_cal DESC
LIMIT 100;
```

### Service Dependencies (Last 24h)
```sql
WITH parent_child AS (
  SELECT DISTINCT 
    p.service_name as parent_service,
    c.service_name as child_service
  FROM spans_final p
  JOIN spans_final c ON p.span_id = c.parent_span_id 
    AND p.trace_id = c.trace_id
  WHERE p.start_time_cal >= now() - INTERVAL 1 DAY
    AND c.start_time_cal >= now() - INTERVAL 1 DAY
)
SELECT parent_service, child_service, count() as call_count
FROM parent_child
GROUP BY parent_service, child_service
ORDER BY call_count DESC;
```

### Real-time Event Timeline
```sql
SELECT service_name, event_type, event_time_cal, count() as event_count
FROM span_events 
WHERE tenant = 'default'
  AND event_time_cal >= now() - INTERVAL 5 MINUTE
GROUP BY service_name, event_type, toStartOfMinute(event_time_cal)
ORDER BY event_time_cal DESC;
```

## Retention Policies

| Table | Retention | Purpose |
|-------|-----------|---------|
| spans_raw | 30 days | Raw data for reprocessing |
| spans_final | 30 days | Processed telemetry data |
| metrics | 90 days | Longer retention for trends |
| logs | 30 days | Application logs |
| calibration_anchors | 2 days | Short-term calibration data |
| span_events | 30 days | Real-time event processing |

## Performance Considerations

- All tables use ZSTD compression for storage efficiency
- Primary keys optimized for time-range queries
- Bloom filter indexes on trace_id and span_id for fast lookups
- Materialized views process data in real-time
- TTL policies automatically clean old data