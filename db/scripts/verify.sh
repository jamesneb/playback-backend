#!/bin/bash

# CI verification script for database migrations
# Tests that migrations are idempotent and schema is correct
#
# This script:
# 1. Starts ClickHouse in Docker
# 2. Runs migrations on clean database
# 3. Runs migrations again (should be no-ops)
# 4. Runs smoke tests to verify schema
# 5. Cleans up

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "🧪 Starting migration verification for CI"

# Configuration
CLICKHOUSE_CONTAINER="verify-clickhouse"
CLICKHOUSE_PORT="19000"
CLICKHOUSE_HTTP_PORT="18123"
TEST_DB="telemetry_test"

cleanup() {
    echo "🧹 Cleaning up test environment..."
    docker stop "$CLICKHOUSE_CONTAINER" 2>/dev/null || true
    docker rm "$CLICKHOUSE_CONTAINER" 2>/dev/null || true
}

# Cleanup on exit
trap cleanup EXIT

echo "🐳 Starting ClickHouse container for testing..."
docker run -d \
    --name "$CLICKHOUSE_CONTAINER" \
    -p "$CLICKHOUSE_PORT:9000" \
    -p "$CLICKHOUSE_HTTP_PORT:8123" \
    -e CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1 \
    clickhouse/clickhouse-server:23.8

echo "⏳ Waiting for ClickHouse to be ready..."
for i in {1..30}; do
    if curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/ping" > /dev/null 2>&1; then
        echo "✅ ClickHouse is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ ClickHouse failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done

# Create test environment configuration
TEST_ENV_FILE="$PROJECT_ROOT/test.env.sql"
cat > "$TEST_ENV_FILE" << EOF
-- Test Environment Configuration
SET DB = '$TEST_DB';
CREATE DATABASE IF NOT EXISTS $TEST_DB;
EOF

echo "🔧 Setting up test environment..."
export ENV="test"
export CLICKHOUSE_HOST="localhost:$CLICKHOUSE_PORT"
export CLICKHOUSE_USER="default"
export CLICKHOUSE_PASSWORD=""
export CLICKHOUSE_DB="$TEST_DB"
export MIGRATIONS_PATH="$PROJECT_ROOT/db/migrations"

# Create a temporary config file for testing
TEST_CONFIG="$PROJECT_ROOT/config/environments/test.yaml"
cp "$PROJECT_ROOT/config/environments/local.yaml" "$TEST_CONFIG"

# Update the test config to use our test database settings
sed -i.bak "s/host: \"localhost:9000\"/host: \"localhost:$CLICKHOUSE_PORT\"/" "$TEST_CONFIG"
sed -i.bak "s/database: \"telemetry\"/database: \"$TEST_DB\"/" "$TEST_CONFIG"
sed -i.bak "s/password: \"admin123\"/password: \"\"/" "$TEST_CONFIG"

echo "🚀 Running migrations (first time)..."
cd "$PROJECT_ROOT"
go run db/scripts/migrate.go

echo "🔄 Running migrations again (should be idempotent)..."
go run db/scripts/migrate.go

echo "🔍 Verifying schema with smoke tests..."

# Test 1: Check that core tables exist
echo "   Testing core tables existence..."
EXPECTED_TABLES=("spans_raw" "spans_final" "metrics" "logs" "calibration_models" "calibration_anchors" "span_events" "schema_migrations")

for table in "${EXPECTED_TABLES[@]}"; do
    result=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "EXISTS TABLE $TEST_DB.$table")
    if [ "$result" != "1" ]; then
        echo "❌ Table $table does not exist"
        exit 1
    fi
done
echo "   ✅ All expected tables exist"

# Test 2: Check that materialized views exist
echo "   Testing materialized views..."
MV_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM system.tables WHERE database = '$TEST_DB' AND engine = 'MaterializedView'")
if [ "$MV_COUNT" -lt 2 ]; then
    echo "❌ Expected at least 2 materialized views, found $MV_COUNT"
    exit 1
fi
echo "   ✅ Materialized views created"

# Test 3: Check spans_final has correct ORDER BY
echo "   Testing spans_final ORDER BY..."
ORDER_BY=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT sorting_key FROM system.tables WHERE database = '$TEST_DB' AND name = 'spans_final'")
if [[ "$ORDER_BY" != *"tenant"* ]] || [[ "$ORDER_BY" != *"start_time_cal"* ]]; then
    echo "❌ spans_final does not have expected ORDER BY clause: $ORDER_BY"
    exit 1
fi
echo "   ✅ spans_final has correct ORDER BY"

# Test 4: Test a simple insert and materialized view trigger
echo "   Testing data flow..."
curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "
INSERT INTO $TEST_DB.spans_raw 
(service_name, trace_id, raw_otlp, ingested_at) VALUES 
('test-service', 'test-trace-123', '{\"resourceSpans\":[{\"scopeSpans\":[{\"spans\":[{\"traceId\":\"test-trace-123\",\"spanId\":\"test-span-456\",\"name\":\"test-operation\",\"startTimeUnixNano\":1693843200000000000,\"endTimeUnixNano\":1693843201000000000}]}]}]}', now64())
"

# Wait a moment for materialized view to process
sleep 2

# Check if data flowed through materialized view
SPAN_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_final WHERE trace_id = 'test-trace-123'")
if [ "$SPAN_COUNT" != "1" ]; then
    echo "❌ Expected 1 span in spans_final, found $SPAN_COUNT"
    exit 1
fi

EVENT_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.span_events WHERE trace_id_hash = cityHash64('test-trace-123')")
if [ "$EVENT_COUNT" != "2" ]; then
    echo "❌ Expected 2 events in span_events (start/end), found $EVENT_COUNT"
    exit 1
fi
echo "   ✅ Data flow working correctly"

# Test 5: Check schema_migrations table
echo "   Testing migration tracking..."
MIGRATION_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.schema_migrations")
if [ "$MIGRATION_COUNT" -lt 4 ]; then
    echo "❌ Expected at least 4 migration records, found $MIGRATION_COUNT"
    exit 1
fi
echo "   ✅ Migration tracking working"

# ============================================================================
# GUARDRAIL QUERIES - Additional verification of data integrity
# ============================================================================

echo "🛡️  Running guardrail queries for data integrity..."

# Guardrail 1: Row count validation between spans_raw and spans_final
echo "   Testing row count parity between spans_raw and spans_final..."
RAW_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_raw")
FINAL_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_final")
if [ "$RAW_COUNT" != "$FINAL_COUNT" ]; then
    echo "❌ Row count mismatch: spans_raw($RAW_COUNT) != spans_final($FINAL_COUNT)"
    exit 1
fi
echo "   ✅ Row count parity maintained"

# Guardrail 2: NULL validation on calibrated columns
echo "   Testing for NULLs in calibrated timestamp columns..."
NULL_START_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_final WHERE start_time_cal IS NULL")
NULL_END_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_final WHERE end_time_cal IS NULL")
if [ "$NULL_START_COUNT" != "0" ] || [ "$NULL_END_COUNT" != "0" ]; then
    echo "❌ Found NULL calibrated timestamps: start_time_cal($NULL_START_COUNT), end_time_cal($NULL_END_COUNT)"
    exit 1
fi
echo "   ✅ No NULL calibrated timestamps"

# Guardrail 3: Materialized view parity check
echo "   Testing materialized view event generation..."
SPANS_WITH_EVENTS=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "
SELECT count() FROM $TEST_DB.spans_final sf
WHERE EXISTS (
    SELECT 1 FROM $TEST_DB.span_events se 
    WHERE se.trace_id_hash = cityHash64(sf.trace_id)
    AND se.span_id_hash = cityHash64(sf.span_id)
)")
TOTAL_SPANS=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "SELECT count() FROM $TEST_DB.spans_final")
if [ "$SPANS_WITH_EVENTS" != "$TOTAL_SPANS" ]; then
    echo "❌ Materialized view parity issue: $SPANS_WITH_EVENTS/$TOTAL_SPANS spans have events"
    exit 1
fi
echo "   ✅ All spans have corresponding events"

# Guardrail 4: Duplicate prevention check
echo "   Testing duplicate prevention in spans_final..."
DUPLICATE_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "
SELECT count() FROM (
    SELECT trace_id, span_id, count() as cnt
    FROM $TEST_DB.spans_final
    GROUP BY trace_id, span_id
    HAVING cnt > 1
)")
if [ "$DUPLICATE_COUNT" != "0" ]; then
    echo "❌ Found $DUPLICATE_COUNT duplicate span records"
    exit 1
fi
echo "   ✅ No duplicate spans detected"

# Guardrail 5: Time consistency validation
echo "   Testing time consistency (end >= start)..."
TIME_INCONSISTENT_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "
SELECT count() FROM $TEST_DB.spans_final 
WHERE end_time_cal < start_time_cal")
if [ "$TIME_INCONSISTENT_COUNT" != "0" ]; then
    echo "❌ Found $TIME_INCONSISTENT_COUNT spans with end_time < start_time"
    exit 1
fi
echo "   ✅ Time consistency validated"

# Guardrail 6: HLC monotonicity check
echo "   Testing HLC monotonicity within traces..."
HLC_VIOLATION_COUNT=$(curl -s "http://localhost:$CLICKHOUSE_HTTP_PORT/" -d "
WITH ordered_spans AS (
    SELECT trace_id, hlc_wall_ns, hlc_logical,
           lag(hlc_wall_ns) OVER (PARTITION BY trace_id ORDER BY start_time_cal) as prev_wall,
           lag(hlc_logical) OVER (PARTITION BY trace_id ORDER BY start_time_cal) as prev_logical
    FROM $TEST_DB.spans_final
)
SELECT count() FROM ordered_spans
WHERE prev_wall IS NOT NULL 
  AND (hlc_wall_ns < prev_wall OR (hlc_wall_ns = prev_wall AND hlc_logical <= prev_logical))")
if [ "$HLC_VIOLATION_COUNT" != "0" ]; then
    echo "❌ Found $HLC_VIOLATION_COUNT HLC monotonicity violations"
    exit 1
fi
echo "   ✅ HLC monotonicity maintained"

# Cleanup test files
rm -f "$TEST_ENV_FILE" "$TEST_CONFIG" "$TEST_CONFIG.bak"

echo ""
echo "🎉 All verification tests passed!"
echo "   ✅ Migrations are idempotent"
echo "   ✅ Schema is correct"
echo "   ✅ Data flow is working"
echo "   ✅ Migration tracking is functional"
echo "   🛡️  Data integrity guardrails verified"
echo ""
echo "Migration system is ready for production use!"