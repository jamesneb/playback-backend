-- Service user accounts for production deployment
-- Creates users for each service component with appropriate role assignments

USE ${DB};

-- ============================================================================
-- ROLES (add compiler role)
-- ============================================================================

-- Compiler service role - can read telemetry data and write compiled playbooks
CREATE ROLE IF NOT EXISTS compiler;

-- ============================================================================
-- COMPILER PERMISSIONS
-- ============================================================================

-- Compiler needs read access to telemetry data for analysis
GRANT SELECT ON ${DB}.spans_final TO compiler;
GRANT SELECT ON ${DB}.metrics TO compiler;
GRANT SELECT ON ${DB}.logs TO compiler;
GRANT SELECT ON ${DB}.span_events TO compiler;
GRANT SELECT ON ${DB}.calibration_models TO compiler; -- For accurate timing

-- Compiler needs access to calibration data for precise timing
GRANT SELECT ON ${DB}.calibration_anchors TO compiler;
GRANT SELECT ON ${DB}.calibration_watermarks TO compiler;

-- ============================================================================
-- SERVICE USERS
-- ============================================================================

-- Main application service user
CREATE USER IF NOT EXISTS 'playback_app' IDENTIFIED BY '${PLAYBACK_APP_PASSWORD}';
GRANT app_service TO 'playback_app';

-- Kinesis consumer service user  
CREATE USER IF NOT EXISTS 'kinesis_consumer' IDENTIFIED BY '${KINESIS_CONSUMER_PASSWORD}';
GRANT app_service TO 'kinesis_consumer';

-- Calibration service user
CREATE USER IF NOT EXISTS 'calibrator_service' IDENTIFIED BY '${CALIBRATOR_PASSWORD}';
GRANT calibrator TO 'calibrator_service';

-- Orchestrator service user
CREATE USER IF NOT EXISTS 'orchestrator_service' IDENTIFIED BY '${ORCHESTRATOR_PASSWORD}';
GRANT orchestrator TO 'orchestrator_service';

-- Compiler service user
CREATE USER IF NOT EXISTS 'compiler_service' IDENTIFIED BY '${COMPILER_PASSWORD}';
GRANT compiler TO 'compiler_service';

-- ============================================================================
-- EXTERNAL ACCESS USERS
-- ============================================================================

-- Dashboard/analytics read-only user
CREATE USER IF NOT EXISTS 'dashboard_readonly' IDENTIFIED BY '${DASHBOARD_PASSWORD}';
GRANT readonly TO 'dashboard_readonly';

-- External analytics tools
CREATE USER IF NOT EXISTS 'analytics_readonly' IDENTIFIED BY '${ANALYTICS_PASSWORD}';
GRANT readonly TO 'analytics_readonly';

-- ============================================================================
-- OPERATIONAL USERS
-- ============================================================================

-- Database administrator
CREATE USER IF NOT EXISTS 'telemetry_dba' IDENTIFIED BY '${DBA_PASSWORD}';
GRANT telemetry_admin TO 'telemetry_dba';

-- Migration runner (for CI/CD)
CREATE USER IF NOT EXISTS 'migration_runner' IDENTIFIED BY '${MIGRATION_PASSWORD}';
GRANT telemetry_admin TO 'migration_runner';

-- ============================================================================
-- USER SETTINGS AND QUOTAS
-- ============================================================================

-- Production quotas to prevent runaway queries
CREATE QUOTA IF NOT EXISTS 'app_service_quota' 
FOR INTERVAL 1 HOUR MAX queries = 10000, result_rows = 1000000000, read_rows = 10000000000;

CREATE QUOTA IF NOT EXISTS 'readonly_quota'
FOR INTERVAL 1 HOUR MAX queries = 1000, result_rows = 100000000, read_rows = 1000000000;

CREATE QUOTA IF NOT EXISTS 'compiler_quota'
FOR INTERVAL 1 HOUR MAX queries = 5000, result_rows = 500000000, read_rows = 5000000000;

CREATE QUOTA IF NOT EXISTS 'admin_quota'
FOR INTERVAL 1 HOUR MAX queries = 5000, result_rows = 10000000000, read_rows = 100000000000;

-- Apply quotas to users
ALTER USER 'playback_app' SETTINGS QUOTA 'app_service_quota';
ALTER USER 'kinesis_consumer' SETTINGS QUOTA 'app_service_quota';
ALTER USER 'calibrator_service' SETTINGS QUOTA 'app_service_quota';
ALTER USER 'compiler_service' SETTINGS QUOTA 'compiler_quota';
ALTER USER 'dashboard_readonly' SETTINGS QUOTA 'readonly_quota';
ALTER USER 'analytics_readonly' SETTINGS QUOTA 'readonly_quota';
ALTER USER 'telemetry_dba' SETTINGS QUOTA 'admin_quota';

-- ============================================================================
-- SETTINGS PROFILES
-- ============================================================================

-- Optimize settings for different workloads
CREATE SETTINGS PROFILE IF NOT EXISTS 'app_service_profile' SETTINGS
    max_memory_usage = '4GB',
    max_execution_time = 60,
    max_result_rows = 1000000,
    readonly = 0;

CREATE SETTINGS PROFILE IF NOT EXISTS 'readonly_profile' SETTINGS  
    max_memory_usage = '2GB',
    max_execution_time = 300,
    max_result_rows = 10000000,
    readonly = 1;

CREATE SETTINGS PROFILE IF NOT EXISTS 'calibrator_profile' SETTINGS
    max_memory_usage = '8GB', 
    max_execution_time = 600,
    max_result_rows = 100000000,
    readonly = 0;

CREATE SETTINGS PROFILE IF NOT EXISTS 'compiler_profile' SETTINGS
    max_memory_usage = '16GB',  -- Compiler may need more memory for complex analysis
    max_execution_time = 1800,  -- 30 minutes for deep trace analysis
    max_result_rows = 50000000,
    readonly = 1;

-- Apply profiles to users
ALTER USER 'playback_app' SETTINGS PROFILE 'app_service_profile';
ALTER USER 'kinesis_consumer' SETTINGS PROFILE 'app_service_profile';  
ALTER USER 'calibrator_service' SETTINGS PROFILE 'calibrator_profile';
ALTER USER 'compiler_service' SETTINGS PROFILE 'compiler_profile';
ALTER USER 'dashboard_readonly' SETTINGS PROFILE 'readonly_profile';
ALTER USER 'analytics_readonly' SETTINGS PROFILE 'readonly_profile';