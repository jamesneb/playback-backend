# calibration_models

Clock drift models for each data source, used to calibrate timestamps across distributed systems.

## Purpose

- Store clock drift parameters per source system
- Enable timestamp calibration for accurate causal ordering
- Support per-tenant clock calibration models
- Track calibration model versions and updates

## Schema

| Column | Type | Description |
|--------|------|-------------|
| `tenant` | LowCardinality(String) | Multi-tenant isolation key |
| `source_id` | LowCardinality(String) | Source system identifier |
| `producer_id` | LowCardinality(String) | Data producer within source |
| `updated_at` | DateTime64(9) | When model was last updated |
| `offset_ns` | Int32 | Clock offset at time t_now (nanoseconds) |
| `drift_ppm` | Int32 | Clock drift rate (parts per million) |
| `jitter_ns_p95` | UInt32 | 95th percentile jitter for uncertainty |
| `epoch` | LowCardinality(String) | Model version/hash identifier |

## Engine

`ReplacingMergeTree(updated_at)` - Latest model per (tenant, source_id, producer_id)

## Indexes

- **Primary Key**: `(tenant, source_id, producer_id)`
- **Partition**: By `tenant` for multi-tenant isolation

## Clock Calibration Formula

```
calibrated_time = raw_time + offset_ns + (drift_ppm * (raw_time - model_time) / 1_000_000)
uncertainty = jitter_ns_p95 + |drift_ppm * time_delta / 1_000_000|
```

## Common Queries

### Get Latest Model for Source
```sql
SELECT * FROM calibration_models
WHERE tenant = 'default' 
  AND source_id = 'api-server-1'
  AND producer_id = 'main'
ORDER BY updated_at DESC
LIMIT 1;
```

### Sources Needing Calibration Update
```sql
SELECT source_id, producer_id, updated_at,
       now() - updated_at as age
FROM calibration_models
WHERE tenant = 'default'
  AND updated_at < now() - INTERVAL 1 HOUR
ORDER BY age DESC;
```

### Drift Analysis
```sql
SELECT source_id, 
       avg(abs(drift_ppm)) as avg_drift_ppm,
       max(abs(drift_ppm)) as max_drift_ppm,
       count() as model_updates
FROM calibration_models
WHERE tenant = 'default'
  AND updated_at >= now() - INTERVAL 1 DAY
GROUP BY source_id
ORDER BY avg_drift_ppm DESC;
```

## Model Updates

Models are updated by the calibration service based on:
- Clock synchronization beacons
- Cross-service timing constraints (RPC calls, etc.)
- Parent-child span relationships
- Queue message timing

## Performance Notes

- ReplacingMergeTree automatically keeps latest version
- Partitioned by tenant for isolation and performance
- Models cached by calibration service for fast lookup
- Updates are batched to reduce write load