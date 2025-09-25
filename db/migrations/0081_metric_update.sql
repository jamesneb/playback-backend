-- EXP_HISTOGRAM datapoints (parallel arrays)
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_time_unix_nano Array(UInt64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_count            Array(UInt64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_sum              Array(Float64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_scale            Array(Int32);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_zero_count       Array(UInt64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_pos_offset       Array(Int32);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_pos_counts       Array(Array(UInt64));
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_neg_offset       Array(Int32);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS eh_neg_counts       Array(Array(UInt64));

-- SUMMARY datapoints (parallel arrays)
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS s_time_unix_nano Array(UInt64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS s_count           Array(UInt64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS s_sum             Array(Float64);
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS s_quantiles       Array(Array(Float64));
ALTER TABLE telemetry.metrics_stage ADD COLUMN IF NOT EXISTS s_values          Array(Array(Float64));

