# ClickHouse Protobuf Schema Deployment

This directory contains protobuf schema definitions for native OTLP parsing in ClickHouse.

## 📋 Setup Instructions

### 1. **Copy Schemas to ClickHouse Container**

```bash
# Copy all protobuf schemas to ClickHouse format_schemas directory
docker cp db/schemas/ local-clickhouse-1:/var/lib/clickhouse/format_schemas/

# Alternative: Mount as volume in docker-compose.yml
# volumes:
#   - ./db/schemas:/var/lib/clickhouse/format_schemas/schemas:ro
```

### 2. **Verify Schema Installation**

```bash
# Connect to ClickHouse and verify schemas are available
curl "http://localhost:8123" -d "SELECT * FROM system.formats WHERE name = 'Protobuf'"

# Test protobuf parsing (should not error)
curl "http://localhost:8123" -d "SELECT protobufExtractString('', 'test', 'otlp_trace.proto:ResourceSpans') FORMAT JSON"
```

### 3. **Run Migration**

```bash
# Apply the new materialized view migration
cd /path/to/playback-backend
./scripts/migrate.sh 0008_native_protobuf_materialized_view.sql
```

## 🔧 Schema Files

- **`otlp_trace.proto`** - Main OTLP trace message definitions
- **`opentelemetry/proto/common/v1/common.proto`** - Shared data types (KeyValue, AnyValue, etc.)
- **`opentelemetry/proto/resource/v1/resource.proto`** - Resource attribute definitions

## 🎯 Functions Enabled

After deployment, the materialized view can use these ClickHouse functions:

### Protobuf Extraction Functions

```sql
-- Extract string fields
protobufExtractString(raw_otlp_pb, 'scope_spans.0.spans.0.name', 'otlp_trace.proto:ResourceSpans')

-- Extract binary fields (trace IDs, span IDs)  
protobufExtractBytes(raw_otlp_pb, 'scope_spans.0.spans.0.trace_id', 'otlp_trace.proto:ResourceSpans')

-- Extract numeric fields (timestamps)
protobufExtractUInt64(raw_otlp_pb, 'scope_spans.0.spans.0.start_time_unix_nano', 'otlp_trace.proto:ResourceSpans')
```

### Field Paths

| **Field** | **Protobuf Path** | **Description** |
|---|---|---|
| Trace ID | `scope_spans.0.spans.0.trace_id` | 16-byte trace identifier |
| Span ID | `scope_spans.0.spans.0.span_id` | 8-byte span identifier |
| Parent Span ID | `scope_spans.0.spans.0.parent_span_id` | 8-byte parent span identifier |
| Operation Name | `scope_spans.0.spans.0.name` | Span operation name |
| Start Time | `scope_spans.0.spans.0.start_time_unix_nano` | Start time in nanoseconds |
| End Time | `scope_spans.0.spans.0.end_time_unix_nano` | End time in nanoseconds |
| Status Code | `scope_spans.0.spans.0.status.code` | Span status code |
| Service Name | `resource.attributes[?(@.key=="service.name")].value.string_value` | Service name from resource attributes |

## 🚨 Troubleshooting

### Error: "Unknown format Protobuf"
- Schemas not copied to ClickHouse format_schemas directory
- Restart ClickHouse container after copying schemas

### Error: "Cannot parse protobuf"  
- Check protobuf schema syntax with `protoc --decode_raw`
- Verify field paths match actual protobuf structure
- Ensure raw_otlp_pb contains valid protobuf data

### Error: "Schema not found"
- Verify schema file paths in migration match actual file structure
- Check ClickHouse has read permissions for format_schemas directory

## 🔍 Testing

```sql
-- Test JSON vs Protobuf extraction
SELECT 
    format_type,
    trace_id,
    operation_name,
    start_time
FROM telemetry.spans_final 
WHERE service_name = 'order-service'
ORDER BY start_time DESC 
LIMIT 10;

-- Should show real values for both 'json' and 'protobuf' format_type
```

## 📈 Expected Results

After deployment, both JSON and protobuf data should show **real extracted values** instead of placeholders:

- ✅ **JSON**: `trace_id: "abc123..."`, `operation_name: "check_inventory"`
- ✅ **Protobuf**: `trace_id: "def456..."`, `operation_name: "process_payment"`

No more `pb_trace_id` placeholders!