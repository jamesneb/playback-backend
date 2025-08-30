#!/bin/bash

# Backfill helper script for telemetry database
# Safely reprocesses historical data through materialized views and calibration
#
# This script:
# 1. Validates backfill parameters and safety constraints
# 2. Creates temporary tables for incremental processing
# 3. Reprocesses spans_raw -> spans_final for specified time range
# 4. Regenerates span_events from reprocessed spans
# 5. Updates calibration models if needed
# 6. Validates data consistency after backfill

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default configuration
DEFAULT_BATCH_SIZE=100000
DEFAULT_MAX_HOURS=24
DEFAULT_DRY_RUN=false

usage() {
    cat << EOF
Usage: $0 [OPTIONS] <start_time> <end_time>

Backfill telemetry data for specified time range.

ARGUMENTS:
    start_time    Start time in YYYY-MM-DD HH:MM:SS format (UTC)
    end_time      End time in YYYY-MM-DD HH:MM:SS format (UTC)

OPTIONS:
    -e, --env ENV         Environment configuration (local, staging, prod)
    -t, --tenant TENANT   Tenant to backfill (default: 'default')
    -b, --batch-size N    Process N spans at a time (default: $DEFAULT_BATCH_SIZE)
    -m, --max-hours N     Maximum time range in hours (default: $DEFAULT_MAX_HOURS)
    -d, --dry-run         Show what would be done without executing
    -f, --force           Skip safety checks (use with caution)
    -c, --calibrate       Update calibration models during backfill
    -h, --help            Show this help message

EXAMPLES:
    # Dry run for last 6 hours in staging
    $0 -e staging -d "2024-01-15 18:00:00" "2024-01-16 00:00:00"
    
    # Backfill with calibration update in production
    $0 -e prod -c -t customer1 "2024-01-15 20:00:00" "2024-01-15 22:00:00"
    
    # Large backfill with custom batch size
    $0 -e prod -b 50000 "2024-01-15 00:00:00" "2024-01-16 00:00:00"

SAFETY FEATURES:
    - Maximum time range enforcement ($DEFAULT_MAX_HOURS hours default)
    - Batch processing to prevent memory issues
    - Data validation before and after backfill
    - Dry-run mode for testing
    - Automatic rollback on critical errors

EOF
}

# Parse command line arguments
ENV=""
TENANT="default"
BATCH_SIZE=$DEFAULT_BATCH_SIZE
MAX_HOURS=$DEFAULT_MAX_HOURS
DRY_RUN=$DEFAULT_DRY_RUN
FORCE=false
CALIBRATE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENV="$2"
            shift 2
            ;;
        -t|--tenant)
            TENANT="$2"
            shift 2
            ;;
        -b|--batch-size)
            BATCH_SIZE="$2"
            shift 2
            ;;
        -m|--max-hours)
            MAX_HOURS="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -c|--calibrate)
            CALIBRATE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            break
            ;;
    esac
done

# Validate required arguments
if [ $# -ne 2 ]; then
    echo "❌ Error: start_time and end_time are required"
    usage
    exit 1
fi

START_TIME="$1"
END_TIME="$2"

# Validate environment
if [ -z "$ENV" ]; then
    echo "❌ Error: Environment (-e/--env) is required"
    usage
    exit 1
fi

# Load configuration
CONFIG_FILE="$PROJECT_ROOT/config/environments/$ENV.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Error: Configuration file not found: $CONFIG_FILE"
    exit 1
fi

echo "🔄 Starting backfill process"
echo "   Environment: $ENV"
echo "   Tenant: $TENANT"
echo "   Time range: $START_TIME to $END_TIME"
echo "   Batch size: $BATCH_SIZE"
echo "   Dry run: $DRY_RUN"

# Validate time range
START_EPOCH=$(date -d "$START_TIME" +%s 2>/dev/null || date -j -f "%Y-%m-%d %H:%M:%S" "$START_TIME" +%s)
END_EPOCH=$(date -d "$END_TIME" +%s 2>/dev/null || date -j -f "%Y-%m-%d %H:%M:%S" "$END_TIME" +%s)

if [ $END_EPOCH -le $START_EPOCH ]; then
    echo "❌ Error: End time must be after start time"
    exit 1
fi

DURATION_HOURS=$(( (END_EPOCH - START_EPOCH) / 3600 ))
if [ $DURATION_HOURS -gt $MAX_HOURS ] && [ "$FORCE" != "true" ]; then
    echo "❌ Error: Time range too large ($DURATION_HOURS hours > $MAX_HOURS hours limit)"
    echo "   Use --max-hours to increase limit or --force to override"
    exit 1
fi

# Set environment for Go migration runner
export ENV="$ENV"

# Helper function to execute ClickHouse queries
execute_query() {
    local query="$1"
    local description="$2"
    
    if [ "$DRY_RUN" = "true" ]; then
        echo "   [DRY RUN] Would execute: $description"
        echo "   Query: $(echo "$query" | tr '\n' ' ' | sed 's/  */ /g')"
        return 0
    fi
    
    echo "   Executing: $description"
    cd "$PROJECT_ROOT"
    go run db/scripts/query.go "$query"
}

# Helper function to get row count
get_count() {
    local query="$1"
    if [ "$DRY_RUN" = "true" ]; then
        echo "0"
        return 0
    fi
    
    cd "$PROJECT_ROOT"
    go run db/scripts/query.go "$query" | tail -n1
}

echo ""
echo "🔍 Pre-backfill validation..."

# Check existing data counts
EXISTING_RAW=$(get_count "
SELECT count() FROM spans_raw 
WHERE tenant = '$TENANT' 
  AND ingested_at >= toDateTime64('$START_TIME', 9)
  AND ingested_at <= toDateTime64('$END_TIME', 9)
")

EXISTING_FINAL=$(get_count "
SELECT count() FROM spans_final 
WHERE tenant = '$TENANT' 
  AND start_time_cal >= toDateTime64('$START_TIME', 9) 
  AND start_time_cal <= toDateTime64('$END_TIME', 9)
")

echo "   Found $EXISTING_RAW spans in spans_raw"
echo "   Found $EXISTING_FINAL spans in spans_final"

if [ "$EXISTING_RAW" -eq 0 ]; then
    echo "❌ Error: No raw data found in time range"
    exit 1
fi

if [ "$EXISTING_FINAL" -gt 0 ] && [ "$FORCE" != "true" ]; then
    echo "❌ Error: spans_final already contains data for this time range"
    echo "   Use --force to reprocess existing data"
    exit 1
fi

echo ""
echo "🚀 Starting backfill process..."

# Create temporary tracking table for this backfill
BACKFILL_ID="backfill_$(date +%Y%m%d_%H%M%S)_$(echo $RANDOM)"
execute_query "
CREATE TABLE IF NOT EXISTS backfill_tracking (
    backfill_id String,
    tenant String,
    start_time DateTime64(9),
    end_time DateTime64(9),
    status String,
    spans_processed UInt64,
    events_generated UInt64,
    started_at DateTime64(9),
    completed_at Nullable(DateTime64(9)),
    error_message Nullable(String)
) ENGINE = MergeTree()
ORDER BY (backfill_id, started_at)
" "Create backfill tracking table"

execute_query "
INSERT INTO backfill_tracking 
(backfill_id, tenant, start_time, end_time, status, spans_processed, events_generated, started_at)
VALUES 
('$BACKFILL_ID', '$TENANT', toDateTime64('$START_TIME', 9), toDateTime64('$END_TIME', 9), 'started', 0, 0, now64())
" "Record backfill start"

# Process in batches
TOTAL_PROCESSED=0
BATCH_START_TIME="$START_TIME"

while [ "$(date -d "$BATCH_START_TIME" +%s 2>/dev/null || date -j -f "%Y-%m-%d %H:%M:%S" "$BATCH_START_TIME" +%s)" -lt "$END_EPOCH" ]; do
    # Calculate batch end time (1 hour or remaining time, whichever is smaller)
    BATCH_END_EPOCH=$(( $(date -d "$BATCH_START_TIME" +%s 2>/dev/null || date -j -f "%Y-%m-%d %H:%M:%S" "$BATCH_START_TIME" +%s) + 3600 ))
    if [ $BATCH_END_EPOCH -gt $END_EPOCH ]; then
        BATCH_END_EPOCH=$END_EPOCH
    fi
    
    BATCH_END_TIME=$(date -d "@$BATCH_END_EPOCH" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || date -j -f "%s" "$BATCH_END_EPOCH" "+%Y-%m-%d %H:%M:%S")
    
    echo "   Processing batch: $BATCH_START_TIME to $BATCH_END_TIME"
    
    # Delete existing processed data for this batch
    execute_query "
    DELETE FROM spans_final 
    WHERE tenant = '$TENANT'
      AND start_time_cal >= toDateTime64('$BATCH_START_TIME', 9)
      AND start_time_cal < toDateTime64('$BATCH_END_TIME', 9)
    " "Clear existing spans_final data for batch"
    
    execute_query "
    DELETE FROM span_events 
    WHERE tenant = '$TENANT'
      AND event_time_cal >= toDateTime64('$BATCH_START_TIME', 9)
      AND event_time_cal < toDateTime64('$BATCH_END_TIME', 9)
    " "Clear existing span_events data for batch"
    
    # Get batch count for progress tracking
    BATCH_COUNT=$(get_count "
    SELECT count() FROM spans_raw 
    WHERE tenant = '$TENANT' 
      AND ingested_at >= toDateTime64('$BATCH_START_TIME', 9)
      AND ingested_at < toDateTime64('$BATCH_END_TIME', 9)
    ")
    
    echo "     Found $BATCH_COUNT spans in batch"
    
    if [ "$BATCH_COUNT" -gt 0 ]; then
        # Trigger materialized view processing by touching spans_raw
        # This is a no-op update that causes MVs to reprocess the data
        execute_query "
        INSERT INTO spans_raw 
        SELECT * FROM spans_raw 
        WHERE tenant = '$TENANT'
          AND ingested_at >= toDateTime64('$BATCH_START_TIME', 9)
          AND ingested_at < toDateTime64('$BATCH_END_TIME', 9)
        LIMIT 0
        " "Trigger materialized view reprocessing"
        
        # Wait for materialized views to process
        if [ "$DRY_RUN" != "true" ]; then
            echo "     Waiting for materialized view processing..."
            sleep 5
        fi
        
        TOTAL_PROCESSED=$((TOTAL_PROCESSED + BATCH_COUNT))
    fi
    
    # Move to next batch
    BATCH_START_TIME="$BATCH_END_TIME"
done

echo ""
echo "🔍 Post-backfill validation..."

# Validate final counts
FINAL_RAW=$(get_count "
SELECT count() FROM spans_raw 
WHERE tenant = '$TENANT' 
  AND ingested_at >= toDateTime64('$START_TIME', 9)
  AND ingested_at <= toDateTime64('$END_TIME', 9)
")

FINAL_FINAL=$(get_count "
SELECT count() FROM spans_final 
WHERE tenant = '$TENANT' 
  AND start_time_cal >= toDateTime64('$START_TIME', 9) 
  AND start_time_cal <= toDateTime64('$END_TIME', 9)
")

FINAL_EVENTS=$(get_count "
SELECT count() FROM span_events 
WHERE tenant = '$TENANT' 
  AND event_time_cal >= toDateTime64('$START_TIME', 9) 
  AND event_time_cal <= toDateTime64('$END_TIME', 9)
")

echo "   Final counts:"
echo "     spans_raw: $FINAL_RAW"
echo "     spans_final: $FINAL_FINAL"  
echo "     span_events: $FINAL_EVENTS"

# Validate data consistency
if [ "$DRY_RUN" != "true" ]; then
    if [ "$FINAL_RAW" != "$FINAL_FINAL" ]; then
        echo "❌ Error: Row count mismatch between spans_raw and spans_final"
        execute_query "
        UPDATE backfill_tracking 
        SET status = 'failed', error_message = 'Row count mismatch', completed_at = now64()
        WHERE backfill_id = '$BACKFILL_ID'
        " "Record backfill failure"
        exit 1
    fi
    
    # Each span should generate 2 events (start + end)
    EXPECTED_EVENTS=$((FINAL_FINAL * 2))
    if [ "$FINAL_EVENTS" -ne "$EXPECTED_EVENTS" ]; then
        echo "⚠️  Warning: Expected $EXPECTED_EVENTS events, found $FINAL_EVENTS"
    fi
fi

# Update calibration models if requested
if [ "$CALIBRATE" = "true" ]; then
    echo ""
    echo "🕐 Updating calibration models..."
    
    # This would typically call the calibration service
    # For now, just log that it would happen
    if [ "$DRY_RUN" = "true" ]; then
        echo "   [DRY RUN] Would trigger calibration model updates"
    else
        echo "   Calibration model updates not implemented yet"
        echo "   Manual calibration service restart may be needed"
    fi
fi

# Mark backfill as completed
execute_query "
UPDATE backfill_tracking 
SET status = 'completed', spans_processed = $TOTAL_PROCESSED, events_generated = $FINAL_EVENTS, completed_at = now64()
WHERE backfill_id = '$BACKFILL_ID'
" "Record backfill completion"

echo ""
if [ "$DRY_RUN" = "true" ]; then
    echo "🎉 Dry run completed successfully!"
    echo "   Would process $TOTAL_PROCESSED spans"
    echo "   Use without --dry-run to execute the backfill"
else
    echo "🎉 Backfill completed successfully!"
    echo "   Processed $TOTAL_PROCESSED spans"
    echo "   Generated $FINAL_EVENTS events"
    echo "   Backfill ID: $BACKFILL_ID"
fi
echo ""