ALTER TABLE telemetry.tenant_watermarks
    ADD COLUMN IF NOT EXISTS max_event_norm_ns     UInt64 AFTER compiled_watermark_norm_ns,
    ADD COLUMN IF NOT EXISTS max_arrival_ns        UInt64 AFTER max_event_norm_ns,
    ADD COLUMN IF NOT EXISTS safe_watermark_norm_ns UInt64 AFTER max_arrival_ns,
    /* lateness policy + observability */
    ADD COLUMN IF NOT EXISTS lateness_budget_ns    UInt64  DEFAULT 10000000000 AFTER safe_watermark_norm_ns, -- 10s
    ADD COLUMN IF NOT EXISTS lateness_quantile     Float32 DEFAULT 0.999        AFTER lateness_budget_ns,
    ADD COLUMN IF NOT EXISTS p95_late_lag_ns       UInt64  DEFAULT 0            AFTER lateness_quantile,
    ADD COLUMN IF NOT EXISTS p99_late_lag_ns       UInt64  DEFAULT 0            AFTER p95_late_lag_ns,
    ADD COLUMN IF NOT EXISTS late_event_count      UInt64  DEFAULT 0            AFTER p99_late_lag_ns;

ALTER TABLE telemetry.segments_manifest
    ADD COLUMN IF NOT EXISTS safe_closed    UInt8          DEFAULT 0 AFTER sha256_hex,
    ADD COLUMN IF NOT EXISTS safe_closed_at DateTime64(9)  DEFAULT toDateTime64(0,9) AFTER safe_closed;
ALTER TABLE telemetry.clock_calibrations
  ADD COLUMN IF NOT EXISTS tenant_id LowCardinality(String) DEFAULT '' AFTER host_id;

