-- DB SETUP
CREATE DATABASE IF NOT EXISTS telemetry;

-- Enable JSON object type if not already enabled
SET allow_experimental_object_type = 1;

-- Helper functions
CREATE FUNCTION IF NOT EXISTS toDateNs AS (ns) -> toDate(toDateTime64(ns/1e9, 9, 'UTC'));

-- Enum types will be defined inline in table schemas:
-- SpanKind: Enum8('UNSPECIFIED'=0, 'INTERNAL'=1,'SERVER'=2,'CLIENT'=3,'PRODUCER'=4,'CONSUMER'=5)
-- ArrivalClass: Enum8('NOW_SKEW'=1, 'HISTORICAL'=2, 'AMBIGUOUS'=3)
-- MetricType: Enum8('GAUGE'=1,'SUM'=2,'HISTOGRAM'=3,'EXP_HISTOGRAM'=4,'SUMMARY'=5)

