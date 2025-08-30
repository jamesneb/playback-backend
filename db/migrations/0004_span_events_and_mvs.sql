-- Migration 0004: Add span events table and materialized views
-- Creates event-based views for time-series analysis and real-time processing

-- Event table for span start/end events
CREATE TABLE IF NOT EXISTS ${DB}.span_events (
  tenant LowCardinality(String),
  service_name LowCardinality(String),
  trace_id_hash UInt64,
  span_id_hash UInt64,
  event_type Enum8('start'=1,'end'=2),
  event_time_cal DateTime64(9),
  hlc_wall_ns Int64,
  hlc_logical UInt32
) ENGINE = MergeTree
PARTITION BY toDate(event_time_cal)
ORDER BY (tenant, event_time_cal, hlc_wall_ns, hlc_logical, trace_id_hash);

-- Materialized view to generate span events using ARRAY JOIN
CREATE MATERIALIZED VIEW IF NOT EXISTS ${DB}.mv_span_events
TO ${DB}.span_events AS
SELECT
  tenant,
  service_name,
  trace_id_hash,
  span_id_hash,
  if(which = 1, 'start', 'end') AS event_type,
  if(which = 1, start_time_cal, end_time_cal) AS event_time_cal,
  hlc_wall_ns,
  hlc_logical
FROM ${DB}.spans_final
ARRAY JOIN [1, 2] AS which
WHERE (which = 1 AND start_time_cal IS NOT NULL)
   OR (which = 2 AND end_time_cal IS NOT NULL);