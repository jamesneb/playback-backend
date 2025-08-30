-- Migration 0003: Add calibration state management tables
-- Creates tables for clock calibration, drift models, and timing constraints

-- Clock drift models per source/producer
CREATE TABLE IF NOT EXISTS ${DB}.calibration_models (
  tenant LowCardinality(String),
  source_id LowCardinality(String),
  producer_id LowCardinality(String),
  updated_at DateTime64(9),
  offset_ns Int32, -- 0(t_now)
  drift_ppm Int32, -- omega
  jitter_ns_p95 UInt32, -- for uncertainty budget
  epoch LowCardinality(String) -- model version/hash
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY tenant
ORDER BY (tenant, source_id, producer_id);

-- Timing constraints for calibration anchors
CREATE TABLE IF NOT EXISTS ${DB}.calibration_anchors (
  tenant LowCardinality(String),
  kind LowCardinality(String), -- 'rpc', 'queue', 'parent_child', 'beacon'
  src_source LowCardinality(String),
  dst_source LowCardinality(String),
  observed_ns Int64, -- raw delta
  lower_bound_ns Int64, -- modeled constraints
  upper_bound_ns Int64,
  at DateTime64(9)
) ENGINE = MergeTree
PARTITION BY toDate(at)
ORDER BY (tenant, at, kind)
TTL toDateTime(at) + INTERVAL 2 DAY DELETE;

-- Processing watermarks for calibration progress
CREATE TABLE IF NOT EXISTS ${DB}.calibration_watermarks (
  tenant LowCardinality(String),
  scope LowCardinality(String), -- 'tenant' or 'source:{id}'
  watermark_cal_ns Int64, -- "complete through" time
  updated_at DateTime64(9)
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (tenant, scope);

-- Cursor tracking for calibrator ingestion
CREATE TABLE IF NOT EXISTS ${DB}.calibrator_cursors (
  tenant LowCardinality(String),
  last_ingested_at DateTime64(9),
  last_ingest_row_id UUID,
  updated_at DateTime64(9)
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (tenant);