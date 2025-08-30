# Playback Backend

A high-performance distributed system telemetry backend for ingesting, processing, and replaying OpenTelemetry data (traces, metrics, logs) with advanced features for system mapping, clock calibration, and multi-tenant data isolation.

## 🚀 Features

- **Multi-Protocol Ingestion**: HTTP/JSON and gRPC/OTLP endpoints
- **Real-time Processing**: Stream processing via AWS Kinesis with ClickHouse storage
- **System Mapping**: Automatic service dependency discovery and visualization  
- **Request Replay**: Store and replay production traffic patterns for testing
- **Clock Calibration**: Advanced clock drift correction using Hybrid Logical Clocks
- **Multi-Tenant**: Secure tenant isolation for customer data
- **Data Scrubbing**: Configurable PII/sensitive data filtering
- **Scalable Storage**: ClickHouse with compression and automated TTL policies

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   OTLP Clients  │───▶│   Playback   │───▶│    Kinesis      │
│  (Your Services)│    │   Backend    │    │   Streaming     │
└─────────────────┘    └──────────────┘    └─────────────────┘
                              │                       │
                              ▼                       ▼
                       ┌─────────────┐         ┌─────────────┐
                       │ ClickHouse  │◀────────│  Consumer   │
                       │  Database   │         │  Services   │
                       └─────────────┘         └─────────────┘
                              │
                              ▼
                       ┌─────────────┐
                       │   S3 Replay │
                       │   Storage   │
                       └─────────────┘
```

## 🛠️ Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.23+ (for development)
- Make (for build automation)

### Local Development

```bash
# Clone the repository
git clone <your-repo-url>
cd playback-backend

# Start all services (ClickHouse, Redis, LocalStack, etc.)
make start-local

# Check service health
make health

# View logs
make logs

# Stop services
make stop-local
```

The backend will be available at:
- **HTTP API**: http://localhost:8080
- **gRPC OTLP**: localhost:4317
- **Swagger Docs**: http://localhost:8080/swagger/
- **ClickHouse**: http://localhost:8123

### Sending Test Data

```bash
# Send a test trace
curl -X POST http://localhost:8080/api/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "abc123",
    "span_id": "def456", 
    "operation_name": "test-operation",
    "service_name": "test-service",
    "start_time": "2024-01-01T00:00:00Z",
    "duration_ns": 1000000
  }'

# View replay files
curl http://localhost:8080/api/v1/replays/list
```

## 📊 Data Processing Pipeline

### 1. Ingestion
- **HTTP/JSON**: REST API for simple integrations
- **gRPC/OTLP**: Standard OpenTelemetry protocol for high-performance ingestion
- **Multi-tenant**: Automatic tenant isolation and routing

### 2. Stream Processing  
- **Kinesis Streams**: Separate streams for traces, metrics, and logs
- **Real-time**: Sub-second latency from ingestion to storage
- **Fault Tolerant**: Automatic retry and dead letter queues

### 3. Storage & Querying
- **ClickHouse**: Column-oriented database optimized for analytics
- **Compression**: ZSTD compression for efficient storage
- **Indexing**: Optimized for time-range and service-based queries
- **TTL Policies**: Automatic data lifecycle management

### 4. Data Scrubbing & Security

The system includes configurable data scrubbing to protect sensitive customer information:

```yaml
# config/environments/{env}.yaml
security:
  data_scrubbing:
    enabled: true
    rules:
      - field_patterns: ["password", "token", "key", "secret"]
        action: "redact"
        replacement: "[REDACTED]"
      - field_patterns: ["email", "ssn", "credit_card"]  
        action: "hash"
        algorithm: "sha256"
      - attribute_keys: ["user.id", "customer.email"]
        action: "encrypt"
        key_id: "customer-data-key"
```

**Scrubbing Locations**:
- **Ingestion Layer**: Applied before Kinesis streaming
- **Storage Layer**: Additional filtering before ClickHouse writes
- **Query Layer**: Runtime filtering for API responses

## 🗄️ Database Schema

The system uses a sophisticated schema optimized for telemetry data:

```sql
-- Processed spans with full schema and calibration
CREATE TABLE spans_final (
    trace_id String,
    span_id String, 
    tenant LowCardinality(String) DEFAULT 'default',
    service_name LowCardinality(String),
    operation_name String,
    start_time_cal DateTime64(9),
    duration_ns UInt64,
    -- ... additional telemetry fields
) ENGINE = MergeTree()
ORDER BY (tenant, start_time_cal, service_name, trace_id);
```

**Key Features**:
- **Clock Calibration**: Corrects timestamp drift across distributed systems
- **Hybrid Logical Clocks**: Ensures causal ordering of events
- **Multi-tenancy**: Secure data isolation per customer
- **Compression**: 10:1+ compression ratios with ZSTD

See [Database Documentation](db/docs/README.md) for complete schema details.

## 🔧 Configuration

The system uses a layered configuration approach:

1. **Base Configuration**: YAML files in `config/environments/`
2. **Environment Overrides**: Environment variables for deployment-specific values
3. **Runtime Configuration**: Feature flags and dynamic settings

### Environment Variables

```bash
# Core Settings
ENV=local|dev|staging|prod
LOG_LEVEL=debug|info|warn|error
PORT=8080

# Database
CLICKHOUSE_HOST=localhost:9000
CLICKHOUSE_DB=telemetry
CLICKHOUSE_USER=admin
CLICKHOUSE_PASSWORD=your-secure-password

# Streaming  
AWS_REGION=us-east-1
KINESIS_STREAM_TRACES=telemetry-traces
KINESIS_STREAM_METRICS=telemetry-metrics
KINESIS_STREAM_LOGS=telemetry-logs

# Security
JWT_SECRET=your-jwt-secret
ENABLE_AUTH=true|false
```

## 🧪 Testing

```bash
# Run all tests with coverage
make test

# Run specific package tests
make test-package pkg=./internal/handlers

# Run benchmarks
make test-bench

# Run with race detection
make test-race

# Generate coverage report
make test-coverage
```

## 🚀 Deployment

### Development
```bash
make deploy-dev
```

### Staging  
```bash
make deploy-staging
```

### Production
```bash
make deploy-prod
```

All deployments use Terraform for infrastructure as code. See `infrastructure/terraform/` for environment-specific configurations.

## 📈 Monitoring & Observability

The backend includes comprehensive monitoring:

- **Health Checks**: `/api/v1/health` endpoint for load balancer checks
- **Metrics**: Prometheus metrics on `/metrics` endpoint
- **Profiling**: pprof endpoints for performance debugging
- **Tracing**: Self-instrumented with OpenTelemetry
- **Logging**: Structured JSON logging with configurable levels

## 🔒 Security

### Multi-Tenant Isolation
- **Database**: Row-level security with tenant-based partitioning  
- **API**: JWT-based authentication with tenant claim validation
- **Storage**: S3 bucket policies with tenant-specific prefixes

### Data Protection
- **Encryption**: TLS in transit, AES-256 at rest
- **PII Scrubbing**: Configurable field-level redaction
- **Access Control**: RBAC with service-specific database users
- **Audit Logging**: Complete audit trail for data access

### Rate Limiting
```yaml
api:
  rate_limiting:
    enabled: true
    requests_per_second: 1000
    burst: 2000
```

## 🤝 Contributing

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Run** tests (`make test`)
4. **Commit** changes (`git commit -m 'Add amazing feature'`)  
5. **Push** to branch (`git push origin feature/amazing-feature`)
6. **Open** a Pull Request

### Development Setup

```bash
# Install dependencies
make install-deps

# Run pre-commit checks
make pre-commit

# Format code
make fmt

# Run linter
make lint
```

## 📚 API Documentation

- **Swagger UI**: http://localhost:8080/swagger/
- **OpenAPI Spec**: Available at `/swagger/swagger.json`
- **Database Schema**: [db/docs/README.md](db/docs/README.md)

## 🐛 Troubleshooting

### Common Issues

**ClickHouse Connection Failed**
```bash
# Check if ClickHouse is running
make health

# Reset local environment  
make clean-local
make start-local
```

**Migration Errors**
```bash
# Run migrations manually
make migrate

# Verify migration status
make verify-migrations
```

**Kinesis Stream Issues**
```bash
# Check LocalStack health
curl http://localhost:4566/_localstack/health

# Recreate streams
make stop-local
make start-local
```

## 📄 License

[Your License Here]

## 🏷️ Version

Current version: 1.0.0

See [CHANGELOG.md](CHANGELOG.md) for release notes.