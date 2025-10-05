# Playback Backend

A high-performance distributed telemetry backend for ingesting, processing, and querying OpenTelemetry data (traces, metrics, logs) with advanced features for system mapping, event replay, and multi-tenant data isolation.

## 🚀 Features

- **Multi-Protocol Ingestion**: HTTP/JSON and gRPC/OTLP endpoints for traces, metrics, and logs
- **Real-time Stream Processing**: AWS Kinesis streaming with ClickHouse analytics storage
- **Type-Safe Configuration**: Comprehensive config system with hot-reload, validation, and multiple provider support
- **Resilience Patterns**: Circuit breakers, rate limiting, dead letter queues with configurable thresholds
- **Observability**: Prometheus metrics, Jaeger tracing, structured logging, health checks
- **Optional Features**: Event replay, system dependency mapping, data export (JSON/CSV/Parquet)
- **Security**: TLS/SSL, JWT authentication with refresh tokens, CORS, trusted proxies
- **Performance**: Connection pooling, compression, keep-alive, efficient batch processing
- **Multi-Tenant**: Complete tenant isolation with per-tenant rate limiting
- **Developer Experience**: Comprehensive testing suite, Swagger docs, type-safe APIs

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   OTLP Clients  │───▶│   HTTP / gRPC    │───▶│    Kinesis      │
│  (Your Services)│    │     Servers      │    │    Streams      │
└─────────────────┘    │                  │    │ (Traces/Metrics)│
                       │  • Validation    │    └─────────────────┘
                       │  • Rate Limiting │           │
                       │  • Circuit Break │           │
                       │  • Auth/CORS     │           ▼
                       └──────────────────┘    ┌─────────────────┐
                                │              │    Consumer     │
                                │              │    Services     │
                                │              └─────────────────┘
                                │                       │
                                ▼                       ▼
                         ┌─────────────┐        ┌─────────────┐
                         │   Metrics   │        │ ClickHouse  │
                         │ (Prometheus)│        │  Analytics  │
                         └─────────────┘        └─────────────┘
                                                        │
                         ┌─────────────┐               │
                         │   Tracing   │               ▼
                         │   (Jaeger)  │        ┌─────────────┐
                         └─────────────┘        │   S3 Replay │
                                                │   Storage   │
                                                └─────────────┘
```

## 🛠️ Quick Start

### Prerequisites

- **Docker & Docker Compose** - For local dependencies
- **Go 1.24+** - For development
- **Make** - For build automation

### Local Development

```bash
# Clone the repository
git clone https://github.com/jamesneb/playback-backend
cd playback-backend

# Start infrastructure (ClickHouse, Redis, LocalStack, etc.)
make start-local

# Run the server
make run

# In another terminal, check health
curl http://localhost:8080/api/v1/health

# View Swagger documentation
open http://localhost:8080/swagger

# Stop services
make stop-local
```

The backend will be available at:
- **HTTP API**: http://localhost:8080
- **gRPC OTLP**: localhost:4317
- **Swagger Docs**: http://localhost:8080/swagger
- **Metrics**: http://localhost:9090/metrics
- **ClickHouse**: http://localhost:8123

### Sending Test Data

```bash
# Send a test trace via HTTP
curl -X POST http://localhost:8080/api/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id": "abc123def456",
    "span_id": "span789",
    "operation_name": "test-operation",
    "service_name": "test-service",
    "start_time": "2024-01-01T00:00:00Z",
    "duration_ns": 1000000
  }'

# Query traces
curl http://localhost:8080/api/v1/traces?service=test-service

# Check metrics
curl http://localhost:9090/metrics
```

## 📦 Configuration

The system uses a **layered, type-safe configuration system** with hot-reload support:

### Configuration Layers (Priority Order)

1. **Defaults** - Hardcoded sensible defaults in code
2. **Environment Files** - `.env` files for base configuration
3. **Environment Variables** - Runtime overrides
4. **AWS Secrets Manager** - Secure secrets (optional)

Later layers override earlier ones, allowing flexible deployment patterns.

### Core Configuration

```bash
# Application
APP_NAME=playback
APP_ENVIRONMENT=local|dev|staging|prod
APP_LOG_LEVEL=debug|info|warn|error
APP_LOG_FORMAT=json|console

# HTTP Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8080
HTTP_ENABLE_CORS=true
HTTP_RATE_LIMIT_RPS=1000
HTTP_RATE_LIMIT_BURST=2000

# gRPC Server
GRPC_SERVER_PORT=4317
GRPC_MAX_RECEIVE_SIZE=16777216  # 16MB
GRPC_MAX_SEND_SIZE=16777216
GRPC_MAX_REQUESTS_PER_SECOND=100

# ClickHouse
CLICKHOUSE_HOST=localhost
CLICKHOUSE_PORT=9000
CLICKHOUSE_DATABASE=telemetry
CLICKHOUSE_USER=default
CLICKHOUSE_PASSWORD=

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0
REDIS_POOL_SIZE=10

# AWS Kinesis
KINESIS_REGION=us-east-1
KINESIS_STREAM_TRACES=telemetry-traces
KINESIS_STREAM_METRICS=telemetry-metrics
KINESIS_STREAM_LOGS=telemetry-logs

# S3 Storage
S3_REGION=us-east-1
S3_BUCKET=telemetry-replay
S3_ENABLE_SSE=true

# Monitoring
MONITORING_ENABLE_METRICS=true
MONITORING_METRICS_PORT=9090
MONITORING_ENABLE_JAEGER=false
MONITORING_JAEGER_SAMPLING_RATE=10  # 10%

# Circuit Breaker
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_BREAKER_TIMEOUT=5s
CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=50  # 50%

# Dead Letter Queue
DLQ_ENABLED=true
DLQ_QUEUE_NAME=failed-events-dlq
DLQ_REGION=us-east-1
```

### TLS/SSL Configuration

```bash
# HTTP TLS
HTTP_TLS_ENABLED=true
HTTP_TLS_CERT_FILE=/path/to/cert.pem
HTTP_TLS_KEY_FILE=/path/to/key.pem
HTTP_TLS_MIN_VERSION=1.2

# gRPC TLS
GRPC_TLS_ENABLED=true
GRPC_TLS_CERT_FILE=/path/to/grpc-cert.pem
GRPC_TLS_KEY_FILE=/path/to/grpc-key.pem
```

### JWT Authentication

```bash
HTTP_ENABLE_AUTH=true
HTTP_JWT_SECRET=your-256-bit-secret
HTTP_JWT_EXPIRY=24h
HTTP_JWT_REFRESH_WINDOW=168h  # 7 days
HTTP_JWT_ISSUER=playback-backend
HTTP_JWT_AUDIENCE=playback-api
```

### Feature Flags

```bash
# Event Replay
FEATURES_ENABLE_REPLAY=true
FEATURES_REPLAY_DURATION=1h
FEATURES_REPLAY_BUFFER_SIZE=10485760  # 10MB

# System Dependency Map
FEATURES_ENABLE_SYSTEM_MAP=true
FEATURES_MAP_REFRESH_INTERVAL=5m
FEATURES_MAP_MAX_NODES=1000

# Data Export
FEATURES_ENABLE_DATA_EXPORT=true
FEATURES_EXPORT_FORMATS=json,csv,parquet
FEATURES_EXPORT_MAX_SIZE=104857600  # 100MB
```

### Configuration Hot-Reload

The system watches for configuration changes and reloads automatically:

```go
// Subscribe to config changes
mgr.Subscribe("http-server", func(old, new config.Snapshot) {
    if old.HTTP.Port != new.HTTP.Port {
        log.Printf("HTTP port changed: %d -> %d", old.HTTP.Port, new.HTTP.Port)
        // Trigger server restart or graceful reload
    }
})
```

For complete configuration documentation, see:
- **Config Package Docs**: View in pkgsite at `http://localhost:8080` (run `pkgsite -open .`)
- **Environment Variables**: [internal/config/doc.go](internal/config/doc.go)
- **Validation Rules**: Each section's `constants.go` file

## 🔄 Data Processing Pipeline

### 1. Ingestion Layer
- **HTTP/JSON API**: REST endpoints for simple integrations
- **gRPC/OTLP**: Native OpenTelemetry protocol for high-throughput
- **Rate Limiting**: Per-tenant throttling with configurable RPS and burst
- **Validation**: Schema validation and content-type verification

### 2. Stream Processing
- **Kinesis Streams**: Separate streams for traces, metrics, and logs
- **Resilience**: Circuit breakers with failure rate thresholds
- **Dead Letter Queue**: Failed events routed to DLQ with retry logic
- **Monitoring**: Per-stream metrics and tracing

### 3. Storage & Analytics
- **ClickHouse**: Column-oriented storage optimized for time-series analytics
- **Redis**: Caching layer for hot data and session state
- **S3**: Long-term storage for replay data and cold archives
- **Batch Processing**: Efficient bulk inserts with compression

### 4. Query & Visualization
- **REST API**: Query traces, metrics, and logs via HTTP
- **Grafana Integration**: Pre-built dashboards (planned)
- **Jaeger UI**: Distributed tracing visualization
- **Custom Queries**: Direct ClickHouse SQL access

## 🗄️ Database Schema

ClickHouse tables optimized for telemetry data:

```sql
-- Traces table with tenant partitioning
CREATE TABLE traces (
    trace_id FixedString(32),
    span_id FixedString(16),
    parent_span_id FixedString(16),
    service_name LowCardinality(String),
    operation_name String,
    start_time DateTime64(9, 'UTC'),
    end_time DateTime64(9, 'UTC'),
    duration_ns UInt64,
    status_code UInt8,
    tenant LowCardinality(String) DEFAULT 'default',
    attributes Map(String, String),
    resource_attributes Map(String, String),
    INDEX idx_service service_name TYPE bloom_filter GRANULARITY 1
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(start_time)
ORDER BY (tenant, start_time, service_name, trace_id)
TTL start_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Metrics table
CREATE TABLE metrics (
    metric_name LowCardinality(String),
    metric_type Enum8('gauge' = 1, 'counter' = 2, 'histogram' = 3),
    value Float64,
    timestamp DateTime64(9, 'UTC'),
    tenant LowCardinality(String) DEFAULT 'default',
    labels Map(String, String),
    resource_attributes Map(String, String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant, metric_name, timestamp)
TTL timestamp + INTERVAL 90 DAY;

-- Logs table
CREATE TABLE logs (
    timestamp DateTime64(9, 'UTC'),
    severity_number UInt8,
    severity_text LowCardinality(String),
    body String,
    service_name LowCardinality(String),
    trace_id FixedString(32),
    span_id FixedString(16),
    tenant LowCardinality(String) DEFAULT 'default',
    attributes Map(String, String),
    resource_attributes Map(String, String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (tenant, timestamp, service_name)
TTL timestamp + INTERVAL 14 DAY;
```

## 🧪 Testing

```bash
# Run all tests with coverage
make test

# Run specific package tests
make test-package pkg=./internal/handlers

# Run benchmarks
make benchmark

# Run with race detection
make test-race

# Generate coverage report
make coverage

# Integration tests (requires Docker)
make test-integration
```

### Test Coverage

- **Unit Tests**: Core business logic and handlers
- **Integration Tests**: End-to-end API flows with real dependencies
- **Benchmark Tests**: Performance testing for critical paths
- **Error Scenarios**: Comprehensive error handling and edge cases

## 📈 Monitoring & Observability

### Prometheus Metrics

Available at `http://localhost:9090/metrics`:

```
# HTTP request metrics
http_requests_total{method="POST", endpoint="/api/v1/traces", status="200"}
http_request_duration_seconds{method="POST", endpoint="/api/v1/traces"}

# gRPC metrics
grpc_server_handled_total{grpc_service="otlp.TraceService", grpc_method="Export"}
grpc_server_handling_seconds{grpc_service="otlp.TraceService"}

# Circuit breaker metrics
circuit_breaker_state{name="kinesis"} # 0=closed, 1=half-open, 2=open
circuit_breaker_requests_total{name="kinesis", result="success|failure"}

# Rate limiter metrics
rate_limiter_allowed_total{tenant="default"}
rate_limiter_denied_total{tenant="default"}

# ClickHouse metrics
clickhouse_insert_duration_seconds
clickhouse_insert_errors_total
```

### Jaeger Tracing

When enabled, all requests are traced with configurable sampling:

```bash
MONITORING_ENABLE_JAEGER=true
MONITORING_JAEGER_SERVICE_NAME=playback-backend
MONITORING_JAEGER_SAMPLING_RATE=10  # Sample 10% of traces
```

### Health Checks

```bash
# Basic health
curl http://localhost:8080/api/v1/health

# Detailed component health
curl http://localhost:8080/api/v1/health/detailed
```

## 🔒 Security

### Authentication

JWT-based authentication with refresh tokens:

```bash
# Login to get JWT
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "user", "password": "pass"}'

# Use token in requests
curl http://localhost:8080/api/v1/traces \
  -H "Authorization: Bearer <token>"

# Refresh token before expiry
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer <token>"
```

### CORS

Configurable CORS for browser-based clients:

```bash
HTTP_CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
HTTP_CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE
HTTP_CORS_ALLOW_CREDENTIALS=true
```

### Rate Limiting

Per-tenant rate limiting with token bucket algorithm:

```bash
HTTP_RATE_LIMIT_RPS=1000      # Tokens per second
HTTP_RATE_LIMIT_BURST=2000    # Bucket capacity
```

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t playback-backend:latest .

# Run container
docker run -p 8080:8080 -p 4317:4317 \
  -e HTTP_PORT=8080 \
  -e GRPC_SERVER_PORT=4317 \
  playback-backend:latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  playback:
    image: playback-backend:latest
    ports:
      - "8080:8080"
      - "4317:4317"
    environment:
      - APP_ENVIRONMENT=prod
      - HTTP_PORT=8080
      - CLICKHOUSE_HOST=clickhouse
      - REDIS_HOST=redis
    depends_on:
      - clickhouse
      - redis
```

### Kubernetes

See [deployments/k8s/](deployments/k8s/) for Kubernetes manifests including:
- Deployment with health checks and resource limits
- Service with LoadBalancer
- ConfigMap for configuration
- Secret for sensitive values
- HorizontalPodAutoscaler for scaling

## 📚 API Documentation

- **Swagger UI**: http://localhost:8080/swagger
- **OpenAPI Spec**: http://localhost:8080/swagger/swagger.json
- **Go Package Docs**: Run `pkgsite -open .` to browse internal documentation

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with tests
4. Run tests and linting (`make test && make lint`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Guidelines

- **Write tests** for all new features
- **Update documentation** when changing APIs or configuration
- **Follow Go best practices** and project conventions
- **Run formatters** before committing (`make fmt`)
- **Keep commits focused** and write clear commit messages

## 📄 License

[Your License Here]

## 🔗 Links

- **Documentation**: [internal/config/doc.go](internal/config/doc.go)
- **Contributing**: [CONTRIBUTING.md](CONTRIBUTING.md)
- **Changelog**: [CHANGELOG.md](CHANGELOG.md)
- **Issues**: [GitHub Issues](https://github.com/jamesneb/playback-backend/issues)
