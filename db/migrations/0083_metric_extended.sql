-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

-- ExpHistogram specifics
ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_scale       Int32         DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_zero_count  UInt64        DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_pos_offset  Int32         DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_pos_counts  Array(UInt64) DEFAULT [];


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_neg_offset  Int32         DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS eh_neg_counts  Array(UInt64) DEFAULT [];



-- Summary specifics
ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS s_count      UInt64         DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS s_sum        Float64        DEFAULT 0;


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS s_quantiles  Array(Float64) DEFAULT [];


ALTER TABLE telemetry.metrics_points_raw ADD COLUMN IF NOT EXISTS s_values     Array(Float64) DEFAULT [];


CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_metrics_points_exphist
TO telemetry.metrics_points_raw
AS
SELECT
  tenant_id,
  if(indexOf(resource_attr_key,'service.namespace')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.namespace')])     AS service_namespace,
  if(indexOf(resource_attr_key,'service.name')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.name')])               AS service_name,
  if(indexOf(resource_attr_key,'service.version')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.version')])         AS service_version,
  if(indexOf(resource_attr_key,'service.instance.id')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.instance.id')]) AS service_instance_id,
  multiIf(indexOf(resource_attr_key,'host.id')>0,  if(indexOf(resource_attr_key,'host.id')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'host.id')]),
          indexOf(resource_attr_key,'host.name')>0,if(indexOf(resource_attr_key,'host.name')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'host.name')]),
          '')                                                   AS host_id,

  name                 AS metric_name,
  unit,
  type                 AS metric_type,

  eh_t                 AS time_unix_nano,

  0.0                   AS value_f64,
  0                     AS value_i64,

  -- also expose count/sum via the generic hist columns for easy rollups
  eh_c                 AS h_count,
  eh_s                 AS h_sum,
  CAST([] AS Array(Float64)) AS h_bounds,
  CAST([] AS Array(UInt64))  AS h_bucket_counts,

  -- ExpHistogram specifics
  eh_sc                AS eh_scale,
  eh_zc                AS eh_zero_count,
  eh_po                AS eh_pos_offset,
  eh_pc                AS eh_pos_counts,
  eh_no                AS eh_neg_offset,
  eh_nc                AS eh_neg_counts,

  -- Summary columns empty here
  0                    AS s_count,
  0.0                  AS s_sum,
  CAST([] AS Array(Float64)) AS s_quantiles,
  CAST([] AS Array(Float64)) AS s_values,

  toString(map(metric_attr_key, metric_attr_v_str)) AS metric_labels_json,
  ingested_at
FROM telemetry.metrics_stage
ARRAY JOIN
  eh_time_unix_nano AS eh_t,
  eh_count          AS eh_c,
  eh_sum            AS eh_s,
  eh_scale          AS eh_sc,
  eh_zero_count     AS eh_zc,
  eh_pos_offset     AS eh_po,
  eh_pos_counts     AS eh_pc,
  eh_neg_offset     AS eh_no,
  eh_neg_counts     AS eh_nc
WHERE type = 'EXP_HISTOGRAM';

CREATE MATERIALIZED VIEW IF NOT EXISTS telemetry.mv_stage_to_metrics_points_summary
TO telemetry.metrics_points_raw
AS
SELECT
  tenant_id,
  if(indexOf(resource_attr_key,'service.namespace')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.namespace')])     AS service_namespace,
  if(indexOf(resource_attr_key,'service.name')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.name')])               AS service_name,
  if(indexOf(resource_attr_key,'service.version')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.version')])         AS service_version,
  if(indexOf(resource_attr_key,'service.instance.id')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'service.instance.id')]) AS service_instance_id,
  multiIf(indexOf(resource_attr_key,'host.id')>0,  if(indexOf(resource_attr_key,'host.id')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'host.id')]),
          indexOf(resource_attr_key,'host.name')>0,if(indexOf(resource_attr_key,'host.name')=0,'',resource_attr_v_str[indexOf(resource_attr_key,'host.name')]),
          '')                                                   AS host_id,

  name                 AS metric_name,
  unit,
  type                 AS metric_type,

  s_t                  AS time_unix_nano,

  0.0                   AS value_f64,
  0                     AS value_i64,

  -- generic hist cols empty
  0                     AS h_count,
  0.0                   AS h_sum,
  CAST([] AS Array(Float64)) AS h_bounds,
  CAST([] AS Array(UInt64))  AS h_bucket_counts,

  -- exphist empty
  0                     AS eh_scale,
  0                     AS eh_zero_count,
  0                     AS eh_pos_offset,
  CAST([] AS Array(UInt64)) AS eh_pos_counts,
  0                     AS eh_neg_offset,
  CAST([] AS Array(UInt64)) AS eh_neg_counts,

  -- summary specifics
  s_c                   AS s_count,
  s_s                   AS s_sum,
  s_q                   AS s_quantiles,
  s_v                   AS s_values,

  toString(map(metric_attr_key, metric_attr_v_str)) AS metric_labels_json,
  ingested_at
FROM telemetry.metrics_stage
ARRAY JOIN
  s_time_unix_nano AS s_t,
  s_count          AS s_c,
  s_sum            AS s_s,
  s_quantiles      AS s_q,
  s_values         AS s_v
WHERE type = 'SUMMARY';

