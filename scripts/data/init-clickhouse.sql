-- ClickHouse initialization script for telemetry data
-- This script creates the database and runs all schema files

-- Create the main telemetry database
CREATE DATABASE IF NOT EXISTS telemetry;

-- Use the telemetry database
USE telemetry;

-- Note: Individual table schemas are loaded from config/schemas/
-- This file only handles database creation and initialization