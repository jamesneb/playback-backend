# OpenTelemetry Load Testing Examples

This directory contains example services to generate realistic OpenTelemetry data for testing your playback-backend.

## Architecture

```
Load Test → Order Service → [Inventory Check] → [Payment Processing]
              ↓
         Playback Backend (Your API)
```

## Services

### Order Service (`order-service/`)
- **Port**: 8081
- **Endpoints**: 
  - `POST /orders` - Create new order
  - `GET /orders/:id` - Get order by ID
  - `GET /health` - Health check
- **Generates**: HTTP spans, database simulation spans, external service spans
- **Failure Rate**: 10% (realistic failure scenarios)

### Load Test (`load-test/`)
- Configurable requests per second (RPS)
- Realistic order data generation
- Concurrent request handling
- Performance metrics collection

## Quick Start

### 1. Start Your Playback Backend
```bash
cd ~/playback-backend
go run cmd/server/main.go
# Runs on http://localhost:8080
```

### 2. Start Order Service
```bash
cd examples/order-service
go mod tidy
go run main.go
# Runs on http://localhost:8081
```

### 3. Run Load Test
```bash
cd examples/load-test
go run main.go
```

## Expected Data Generation

### At 10 RPS for 1 minute:
- **~600 HTTP requests**
- **~1,800 trace spans** (3 spans per request)  
- **~3,600 metric points** (6 metrics per request)
- **~4,800 log entries** (8 logs per request)
- **Combined data**: ~7.2 MB
- **~120 KB/sec data rate** across all telemetry types

### Scaling Examples (All Three Pillars):

| RPS | Duration | Requests | Traces | Metrics | Logs | Total Data | Data Rate |
|-----|----------|----------|--------|---------|------|------------|-----------|
| 10  | 1 min    | 600      | 1,800  | 3,600   | 4,800| 7.2 MB     | 120 KB/s  |
| 50  | 5 min    | 15,000   | 45,000 | 90,000  | 120K | 180 MB     | 600 KB/s  |
| 100 | 10 min   | 60,000   | 180K   | 360K    | 480K | 720 MB     | 1.2 MB/s  |
| 500 | 1 hour   | 1.8M     | 5.4M   | 10.8M   | 14.4M| 21.6 GB    | 6 MB/s    |

## Real-World Scenarios

### Small Startup (10-50 services)
- **50-200 RPS** across all services
- **~2-10 MB/s** telemetry data (all three pillars)
- **Daily**: 170 GB - 864 GB

### Medium Company (100-500 services)  
- **500-2000 RPS** across all services
- **~20-100 MB/s** telemetry data
- **Daily**: 1.7 TB - 8.6 TB

### Large Enterprise (1000+ services)
- **5000+ RPS** across all services  
- **~200+ MB/s** telemetry data
- **Daily**: 17+ TB

## Customization

Edit `load-test/main.go` to adjust:
```go
config := LoadTestConfig{
    RequestsPerSec:  100,  // Increase for more load
    DurationSec:     300,  // 5 minutes
    MaxConcurrent:   200,  // Higher concurrency
}
```

## Next Steps

1. **Monitor your playback-backend** performance under different loads
2. **Add database persistence** to handle the data volume
3. **Implement trace storage** (PostgreSQL + S3 for large traces)
4. **Add metrics collection** endpoint
5. **Deploy to AWS** for realistic network conditions

## AWS Deployment (Future)

```bash
# Build and push to ECR
docker build -f deployments/Dockerfile -t order-service .
# Deploy to ECS Fargate
# Configure ALB for load balancing
# Add RDS for persistence
```