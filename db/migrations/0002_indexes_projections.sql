-- Migration 0002: Add indexes and projections for query performance
-- Adds bloom filter indexes and any query-specific optimizations

-- Add bloom filter indexes for fast trace/span lookups
ALTER TABLE ${DB}.spans_final
  ADD INDEX IF NOT EXISTS idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 64,
  ADD INDEX IF NOT EXISTS idx_span_id span_id TYPE bloom_filter(0.01) GRANULARITY 64;