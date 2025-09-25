-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

-- Fix normalized views to include essential JSON columns for observability
-- This addresses the oversight where attributes_json and resource_json were excluded,
-- making it impossible to query by HTTP methods, status codes, service info, etc.

-- Drop existing views to recreate them with proper column inclusion
DROP VIEW IF EXISTS telemetry.spans_normalized;
DROP VIEW IF EXISTS telemetry.logs_normalized;
DROP VIEW IF EXISTS telemetry.metrics_points_normalized;

-- Recreate spans_normalized with attributes and resource JSON included
CREATE VIEW IF NOT EXISTS telemetry.spans_normalized AS
SELECT
  s.tenant_id,
  s.service_namespace,
  s.service_name,
  s.service_version,
  s.service_instance_id,
  s.host_id,

  s.trace_id_bin,
  s.span_id_bin,
  s.parent_span_id_bin,

  s.name,
  s.kind,

  s.start_time_unix_nano,
  s.end_time_unix_nano,
  (s.end_time_unix_nano - s.start_time_unix_nano) AS duration_ns,

  s.status_code,
  s.status_message,

  -- Include the essential JSON columns for observability
  s.resource_json,
  s.attributes_json,

  s.ingested_at,

  -- Clock calibration normalization
  (c.a_num * toInt64(s.start_time_unix_nano)) / c.a_den + c.b_ns AS start_norm_ns,
  (c.a_num * toInt64(s.end_time_unix_nano))   / c.a_den + c.b_ns AS end_norm_ns,
  c.version   AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.spans_raw s
ASOF JOIN telemetry.clock_calibrations c
  ON s.host_id = c.host_id AND toInt64(s.start_time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(s.start_time_unix_nano) < c.t1_orig_ns;

-- Recreate logs_normalized with attributes JSON included
CREATE VIEW IF NOT EXISTS telemetry.logs_normalized AS
SELECT
  l.tenant_id,
  l.service_namespace,
  l.service_name,
  l.service_version,
  l.service_instance_id,
  l.host_id,

  l.time_unix_nano,
  l.observed_time_unix_nano,

  l.severity_number,
  l.severity_text,

  l.body_str,
  l.body_json,

  l.trace_id_hex,
  l.span_id_hex,

  -- Include attributes JSON for log filtering and analysis
  l.attributes_json,

  l.ingested_at,

  -- Clock calibration normalization
  (c.a_num * toInt64(l.time_unix_nano)) / c.a_den + c.b_ns AS time_norm_ns,
  c.version AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.logs_raw l
ASOF JOIN telemetry.clock_calibrations c
  ON l.host_id = c.host_id AND toInt64(l.time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(l.time_unix_nano) < c.t1_orig_ns;

-- Recreate metrics_points_normalized with labels JSON included
CREATE VIEW IF NOT EXISTS telemetry.metrics_points_normalized AS
SELECT
  m.tenant_id,
  m.service_namespace,
  m.service_name,
  m.service_version,
  m.service_instance_id,
  m.host_id,

  m.metric_name,
  m.unit,
  m.metric_type,

  m.time_unix_nano,

  m.value_f64,
  m.value_i64,

  m.h_count,
  m.h_sum,
  m.h_bounds,
  m.h_bucket_counts,

  m.eh_scale,
  m.eh_zero_count,
  m.eh_pos_offset,
  m.eh_pos_counts,
  m.eh_neg_offset,
  m.eh_neg_counts,

  m.s_count,
  m.s_sum,
  m.s_quantiles,
  m.s_values,

  -- Include metric labels JSON for dimensional queries
  m.metric_labels_json,

  m.ingested_at,

  -- Clock calibration normalization
  (c.a_num * toInt64(m.time_unix_nano)) / c.a_den + c.b_ns AS time_norm_ns,
  c.version AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.metrics_points_raw m
ASOF JOIN telemetry.clock_calibrations c
  ON m.host_id = c.host_id AND toInt64(m.time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(m.time_unix_nano) < c.t1_orig_ns;