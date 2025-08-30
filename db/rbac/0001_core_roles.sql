-- Core RBAC setup for telemetry system
-- Creates foundational roles and users for different system components

USE ${DB};

-- ============================================================================
-- ROLES
-- ============================================================================

-- Application service role - can ingest data and run basic queries
CREATE ROLE IF NOT EXISTS app_service;

-- Read-only role for dashboards and analytics
CREATE ROLE IF NOT EXISTS readonly;

-- Calibrator service role - can read/write calibration data
CREATE ROLE IF NOT EXISTS calibrator;

-- Orchestrator role - can manage system operations
CREATE ROLE IF NOT EXISTS orchestrator;

-- Admin role - full access for maintenance
CREATE ROLE IF NOT EXISTS telemetry_admin;

-- ============================================================================
-- CORE TABLE PERMISSIONS
-- ============================================================================

-- Application service permissions (ingest + basic queries)
GRANT SELECT, INSERT ON ${DB}.spans_raw TO app_service;
GRANT SELECT ON ${DB}.spans_final TO app_service;
GRANT SELECT, INSERT ON ${DB}.metrics TO app_service;  
GRANT SELECT, INSERT ON ${DB}.logs TO app_service;
GRANT SELECT ON ${DB}.span_events TO app_service;

-- Read-only permissions (dashboards, analytics)
GRANT SELECT ON ${DB}.spans_final TO readonly;
GRANT SELECT ON ${DB}.metrics TO readonly;
GRANT SELECT ON ${DB}.logs TO readonly;
GRANT SELECT ON ${DB}.span_events TO readonly;
-- NO access to raw tables or calibration data

-- Calibrator permissions (clock calibration system)
GRANT SELECT ON ${DB}.spans_raw TO calibrator;
GRANT SELECT ON ${DB}.spans_final TO calibrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ${DB}.calibration_models TO calibrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ${DB}.calibration_anchors TO calibrator;
GRANT SELECT, INSERT, UPDATE, DELETE ON ${DB}.calibration_watermarks TO calibrator;  
GRANT SELECT, INSERT, UPDATE, DELETE ON ${DB}.calibrator_cursors TO calibrator;
-- Can read spans for calibration but not modify them

-- Orchestrator permissions (system management)
GRANT SELECT ON ${DB}.* TO orchestrator;
GRANT INSERT, UPDATE, DELETE ON ${DB}.calibration_watermarks TO orchestrator;
GRANT INSERT, UPDATE, DELETE ON ${DB}.calibrator_cursors TO orchestrator;
-- Can manage calibration state but not modify telemetry data

-- Admin permissions (full access)
GRANT ALL ON ${DB}.* TO telemetry_admin;

-- ============================================================================
-- SCHEMA MIGRATION PERMISSIONS  
-- ============================================================================

-- Migration runner needs to manage schema_migrations table
GRANT SELECT, INSERT ON ${DB}.schema_migrations TO telemetry_admin;
GRANT CREATE, ALTER, DROP ON ${DB}.* TO telemetry_admin;

-- ============================================================================
-- SYSTEM PERMISSIONS
-- ============================================================================

-- Allow roles to see system information needed for operations
GRANT SELECT ON system.tables TO readonly, app_service, calibrator, orchestrator;
GRANT SELECT ON system.columns TO readonly, app_service, calibrator, orchestrator;
GRANT SELECT ON system.parts TO orchestrator, telemetry_admin;
GRANT SELECT ON system.mutations TO orchestrator, telemetry_admin;