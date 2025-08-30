CREATE TABLE IF NOT EXISTS span_events
(
  tenant  LowCardinality(String),
  service_name  LowCardinality(String),
  trace_id_hash UInt64,
  span_id_hash UInt64,
  event_type  Enum8('start'=1,'end'=2),
  event_time_cal  DateTime64(9),
  hlc_wall_ns Int64,
  hlc_logical UInt32
) ENGINE = MergeTree
PARTITION BY toDate(event_time_cal)
ORDER BY (tenant, event_time_cal, hlc_wall_ns, hlc_logical, trace_id_hash);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_span_events
TO span_events AS
SELECT
  tenant,
  service_name,
  trace_id_hash,
  span_id_hash,
  'start' AS event_type,
  start_time_cal AS event_time_cal,
  hlc_wall_ns, hlc_logical
FROM spans_final
UNION ALL
SELECT
  tenant,
  service_name,
  trace_id_hash,
  span_id_hash,
  'end' AS event_type,
  end_time_cal AS event_time_cal,
  hlc_wall_ns, hlc_logical
FROM spans_final;
