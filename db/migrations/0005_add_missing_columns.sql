-- Migration 0005: Add missing columns for application compatibility
-- Adds columns that the application expects but were missing from initial schema

-- Add metric_type column to metrics table
ALTER TABLE ${DB}.metrics 
ADD COLUMN IF NOT EXISTS metric_type LowCardinality(String) DEFAULT 'gauge';

-- Add trace_flags column to logs table  
ALTER TABLE ${DB}.logs 
ADD COLUMN IF NOT EXISTS trace_flags UInt32 DEFAULT 0;

-- Add service_version column to logs table
ALTER TABLE ${DB}.logs 
ADD COLUMN IF NOT EXISTS service_version LowCardinality(String) DEFAULT 'unknown';