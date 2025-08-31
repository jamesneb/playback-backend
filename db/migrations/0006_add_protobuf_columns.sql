-- Migration 0006: Add protobuf support columns
-- Adds the missing protobuf columns to existing tables

-- Add protobuf columns to spans_raw table
ALTER TABLE ${DB}.spans_raw 
ADD COLUMN IF NOT EXISTS raw_otlp_pb String CODEC(ZSTD(3)),
ADD COLUMN IF NOT EXISTS format_type LowCardinality(String) DEFAULT 'json' CODEC(ZSTD(3));

-- Add protobuf columns to spans_final table  
ALTER TABLE ${DB}.spans_final
ADD COLUMN IF NOT EXISTS raw_otlp_pb String CODEC(ZSTD(3)),
ADD COLUMN IF NOT EXISTS format_type LowCardinality(String) DEFAULT 'json' CODEC(ZSTD(3));

-- Optimize tables after schema changes
OPTIMIZE TABLE ${DB}.spans_raw FINAL;
OPTIMIZE TABLE ${DB}.spans_final FINAL;