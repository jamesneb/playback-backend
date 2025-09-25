CREATE TABLE IF NOT EXISTS telemetry.segments_manifest
(
  tenant_id   String,
  t0_norm_ns  Int64,
  t1_norm_ns  Int64,
  segment_id  UUID,
  version     UInt32,
  s3_key      String,
  size_bytes  UInt64,
  sha256_hex  FixedString(64),
  is_overlay  UInt8,
  base_segment_id UUID DEFAULT generateUUIDv4(),
  compiler_rev String,
  created_at  DateTime64(9)
)
ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toDateNs(t0_norm_ns)
ORDER BY (tenant_id, t0_norm_ns, version);

CREATE TABLE IF NOT EXISTS telemetry.tenant_watermarks
(
  tenant_id String,
  ingest_watermark_norm_ns   Int64,
  compiled_watermark_norm_ns Int64,
  updated_at DateTime64(9)
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY tenant_id;

CREATE TABLE IF NOT EXISTS telemetry.compile_jobs
(
  job_id UUID,
  tenant_id String,
  t0_norm_ns Int64,
  t1_norm_ns Int64,
  priority UInt8,
  status LowCardinality(String),  -- QUEUED|RUNNING|DONE|FAILED
  attempt UInt8,
  created_at DateTime64(9),
  updated_at DateTime64(9),
  error String
)
ENGINE = MergeTree
PARTITION BY toDateNs(t0_norm_ns)
ORDER BY (tenant_id, t0_norm_ns, job_id);

