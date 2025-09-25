-- Enable experimental Object type for JSON columns
SET allow_experimental_object_type = 1;

ALTER TABLE telemetry.spans_raw
  MODIFY TTL ingested_at + INTERVAL 90 DAY DELETE;

ALTER TABLE telemetry.logs_raw
  MODIFY TTL ingested_at + INTERVAL 14 DAY DELETE;

ALTER TABLE telemetry.metrics_points_raw
  MODIFY TTL ingested_at + INTERVAL 30 DAY DELETE;

