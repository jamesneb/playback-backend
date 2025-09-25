-- Per-host calibration segments (ORIGINAL host time range)
CREATE TABLE IF NOT EXISTS telemetry.clock_calibrations
(
  host_id    LowCardinality(String),
  t0_orig_ns Int64,
  t1_orig_ns Int64,
  a_num      Int64,    -- skew numerator
  a_den      Int64,    -- skew denominator
  b_ns       Int64,    -- offset
  epsilon_ns Int64,    -- typical abs error
  version    UInt32,
  updated_at DateTime64(9)
)
ENGINE = MergeTree
ORDER BY (host_id, t0_orig_ns)
SETTINGS index_granularity = 8192;

-- Host health/anomalies (operational)
CREATE TABLE IF NOT EXISTS telemetry.host_health
(
  host_id LowCardinality(String),
  last_seen_ns Int64,
  last_anomaly_at DateTime64(9),
  anomaly_kind LowCardinality(String),
  current_offset_ns Int64,
  current_epsilon_ns Int64,
  notes String
)
ENGINE = ReplacingMergeTree(last_seen_ns)
ORDER BY host_id;

-- Normalized views (ASOF join into the right segment)
CREATE VIEW IF NOT EXISTS telemetry.spans_normalized AS
SELECT
  s.* EXCEPT(resource_json, attributes_json, trace_id_hex, span_id_hex, parent_span_id_hex),
  (c.a_num * toInt64(s.start_time_unix_nano)) / c.a_den + c.b_ns AS start_norm_ns,
  (c.a_num * toInt64(s.end_time_unix_nano))   / c.a_den + c.b_ns AS end_norm_ns,
  c.version   AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.spans_raw s
ASOF JOIN telemetry.clock_calibrations c
  ON s.host_id = c.host_id AND toInt64(s.start_time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(s.start_time_unix_nano) < c.t1_orig_ns;

CREATE VIEW IF NOT EXISTS telemetry.logs_normalized AS
SELECT
  l.* EXCEPT(attributes_json),
  (c.a_num * toInt64(l.time_unix_nano)) / c.a_den + c.b_ns AS time_norm_ns,
  c.version AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.logs_raw l
ASOF JOIN telemetry.clock_calibrations c
  ON l.host_id = c.host_id AND toInt64(l.time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(l.time_unix_nano) < c.t1_orig_ns;

CREATE VIEW IF NOT EXISTS telemetry.metrics_points_normalized AS
SELECT
  m.* EXCEPT(metric_labels_json),
  (c.a_num * toInt64(m.time_unix_nano)) / c.a_den + c.b_ns AS time_norm_ns,
  c.version AS cal_version,
  c.epsilon_ns AS uncertainty_ns
FROM telemetry.metrics_points_raw m
ASOF JOIN telemetry.clock_calibrations c
  ON m.host_id = c.host_id AND toInt64(m.time_unix_nano) >= c.t0_orig_ns
WHERE toInt64(m.time_unix_nano) < c.t1_orig_ns;

