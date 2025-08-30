# Database Scripts

This directory contains operational scripts for managing the telemetry database.

## Scripts

### migrate.go
**Purpose**: Database migration runner that applies versioned SQL migration files.

**Usage**:
```bash
# Run migrations using current environment
go run db/scripts/migrate.go

# Run with specific environment  
ENV=staging go run db/scripts/migrate.go

# Docker usage (via docker-compose)
docker-compose run --rm db-migrate
```

**Features**:
- Idempotent - safe to run multiple times
- Environment-aware configuration
- Transaction-based migration tracking
- Automatic database creation
- Variable substitution (${DB} placeholders)

### verify.sh
**Purpose**: CI verification script that tests migration idempotency and data integrity.

**Usage**:
```bash
# Run full verification suite
./db/scripts/verify.sh

# CI usage
docker run --rm verify-script
```

**Tests**:
- Migration idempotency (run twice, no errors)
- Schema validation (tables, views, indexes exist)
- Data flow testing (materialized views work)
- Guardrail queries (data integrity, consistency)

### backfill.sh
**Purpose**: Safely reprocess historical telemetry data with batch processing and validation.

**Usage**:
```bash
# Dry run for testing
./db/scripts/backfill.sh -e staging -d "2024-01-15 18:00:00" "2024-01-16 00:00:00"

# Production backfill with calibration
./db/scripts/backfill.sh -e prod -c -t customer1 "2024-01-15 20:00:00" "2024-01-15 22:00:00"

# Large backfill with custom batch size  
./db/scripts/backfill.sh -e prod -b 50000 "2024-01-15 00:00:00" "2024-01-16 00:00:00"
```

**Safety Features**:
- Maximum time range limits
- Batch processing (prevents memory issues)
- Pre/post validation
- Dry-run mode
- Rollback on errors
- Progress tracking

### query.go
**Purpose**: Helper utility for executing ClickHouse queries from scripts.

**Usage**:
```bash
# Simple query
go run db/scripts/query.go "SELECT count() FROM spans_final"

# Complex query with environment
ENV=prod go run db/scripts/query.go "SELECT service_name, count() FROM spans_final GROUP BY service_name"
```

**Features**:
- Environment-aware database connections
- Tab-separated output (script-friendly)
- Error handling and logging

## Common Workflows

### Development Setup
```bash
# 1. Start local environment
make start-local

# 2. Run migrations
go run db/scripts/migrate.go

# 3. Verify everything works
./db/scripts/verify.sh
```

### Production Deployment
```bash
# 1. Test migrations in staging
ENV=staging go run db/scripts/migrate.go
ENV=staging ./db/scripts/verify.sh

# 2. Deploy to production
ENV=prod go run db/scripts/migrate.go
ENV=prod ./db/scripts/verify.sh

# 3. Monitor for issues
tail -f logs/telemetry.log
```

### Data Recovery/Reprocessing
```bash
# 1. Identify time range needing reprocessing
go run db/scripts/query.go "SELECT min(ingested_at), max(ingested_at) FROM spans_raw WHERE /* your conditions */"

# 2. Test backfill with dry run
./db/scripts/backfill.sh -e prod -d "start_time" "end_time"

# 3. Execute backfill
./db/scripts/backfill.sh -e prod -c "start_time" "end_time"

# 4. Validate results
go run db/scripts/query.go "SELECT count() FROM spans_final WHERE start_time_cal BETWEEN 'start_time' AND 'end_time'"
```

## Environment Configuration

Scripts use the same configuration system as the main application:

- **local**: `config/environments/local.yaml`
- **staging**: `config/environments/staging.yaml`  
- **prod**: `config/environments/prod.yaml`

Set the `ENV` environment variable to choose configuration:
```bash
export ENV=staging
go run db/scripts/migrate.go
```

## Error Handling

All scripts include comprehensive error handling:

- **migrate.go**: Rolls back failed migrations, logs errors
- **verify.sh**: Fails fast with detailed error messages  
- **backfill.sh**: Tracks progress, enables rollback on critical errors
- **query.go**: Provides clear query execution errors

## Monitoring

Track script execution through:

- **Migration tracking**: `schema_migrations` table
- **Backfill tracking**: `backfill_tracking` table  
- **System logs**: Application logs show script execution
- **Metrics**: Database connection and query metrics

## Best Practices

1. **Always test in staging first**
2. **Use dry-run mode for backfills**  
3. **Monitor system resources during large operations**
4. **Keep backfill batch sizes reasonable (< 100k spans)**
5. **Verify data integrity after major operations**
6. **Coordinate with team before production changes**