-- Pair likely client↔server spans in same trace
CREATE VIEW IF NOT EXISTS telemetry.rpc_pairs AS
SELECT
  c.tenant_id,
  c.service_name AS client_service, s.service_name AS server_service,
  c.host_id      AS client_host,   s.host_id      AS server_host,
  c.trace_id_bin,
  c.start_time_unix_nano AS cs,
  s.start_time_unix_nano AS sr,
  s.end_time_unix_nano   AS ss,
  c.end_time_unix_nano   AS cr,
  greatest(c.start_time_unix_nano, s.start_time_unix_nano) AS t_mid_ns,
  c.ingest_ns AS ingest_client_ns, s.ingest_ns AS ingest_server_ns
FROM telemetry.spans_raw c
JOIN telemetry.spans_raw s
  ON  c.tenant_id = s.tenant_id
  AND c.trace_id_bin = s.trace_id_bin
  AND c.kind = 'CLIENT' AND s.kind = 'SERVER'
  AND (s.parent_span_id_bin = c.span_id_bin
       OR JSON_VALUE(c.attributes_json, '$.rpc.system') = JSON_VALUE(s.attributes_json, '$.rpc.system'));

-- Quartets only: δ and θ
CREATE VIEW IF NOT EXISTS telemetry.rpc_quartets AS
SELECT *,
       (cr - cs) - (ss - sr)           AS rtt_ns,       -- δ
       ((sr - cs) + (ss - cr)) / 2     AS theta_ns      -- server offset vs client
FROM telemetry.rpc_pairs
WHERE cs>0 AND sr>0 AND ss>0 AND cr>0 AND cr>=cs AND ss>=sr;

-- Forward-path delay percentiles (approx d_fwd ≈ δ/2)
CREATE TABLE IF NOT EXISTS telemetry.path_delay_stats
(
  tenant_id String,
  client_service LowCardinality(String),
  server_service LowCardinality(String),
  qs AggregateFunction(quantilesExactLow(0.50, 0.95), Float64),  -- store states
  updated_at DateTime64(9)
)
ENGINE = AggregatingMergeTree
PARTITION BY toDate(updated_at)
ORDER BY (tenant_id, client_service, server_service, updated_at);

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.path_delay_stats_mv
TO telemetry.path_delay_stats AS
SELECT
  tenant_id, client_service, server_service,
  quantilesExactLowState(0.50, 0.95)(rtt_ns/2) AS qs,
  now64(9) AS updated_at
FROM telemetry.rpc_quartets
WHERE t_mid_ns >= (toUnixTimestamp64Nano(now64(9)) - 900000000000)
GROUP BY tenant_id, client_service, server_service;

-- Reader view
CREATE VIEW IF NOT EXISTS telemetry.path_delay_stats_read AS
SELECT tenant_id, client_service, server_service,
       quantileExactLowMerge(0.50)(qs) AS p50_fwd_ns,
       quantileExactLowMerge(0.95)(qs) AS p95_fwd_ns,
       max(updated_at) AS updated_at
FROM telemetry.path_delay_stats
GROUP BY tenant_id, client_service, server_service;

