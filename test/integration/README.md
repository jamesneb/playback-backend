# AWS Integration Tests

This directory contains integration tests that verify the playback-backend works correctly with real AWS services including S3, Kinesis, and other AWS resources.

## Overview

The AWS integration tests validate:

- **S3 Operations**: Upload, download, listing, and error handling
- **Kinesis Streams**: Single record puts, batch puts, error scenarios
- **Concurrent Operations**: Multiple simultaneous AWS operations
- **Error Handling**: Proper handling of AWS service errors
- **Real-world Scenarios**: Using actual AWS services instead of mocks

## Prerequisites

### 1. AWS CLI and Credentials

```bash
# Install AWS CLI
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Configure credentials
aws configure
```

### 2. Required Permissions

Your AWS credentials need the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:CreateBucket",
                "s3:DeleteBucket",
                "s3:PutObject",
                "s3:GetObject",
                "s3:DeleteObject",
                "s3:HeadBucket",
                "s3:HeadObject",
                "s3:PutBucketLifecycleConfiguration"
            ],
            "Resource": [
                "arn:aws:s3:::playback-test-*",
                "arn:aws:s3:::playback-test-*/*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "kinesis:CreateStream",
                "kinesis:DeleteStream",
                "kinesis:DescribeStream",
                "kinesis:PutRecord",
                "kinesis:PutRecords"
            ],
            "Resource": "*"
        }
    ]
}
```

## Setup

### Automatic Setup (Recommended)

```bash
# Set up AWS resources and configuration
./test/integration/setup_aws_resources.sh
```

This script will:
- Create a test S3 bucket with lifecycle policies
- Create Kinesis streams for traces, metrics, and logs
- Generate a `.env` file with configuration
- Set up auto-cleanup policies

### Manual Setup

If you prefer manual setup:

1. Create S3 bucket:
   ```bash
   aws s3api create-bucket --bucket playback-test-bucket-$(date +%s) --region us-east-1
   ```

2. Create Kinesis streams:
   ```bash
   aws kinesis create-stream --stream-name test-telemetry-traces --shard-count 1
   aws kinesis create-stream --stream-name test-telemetry-metrics --shard-count 1
   aws kinesis create-stream --stream-name test-telemetry-logs --shard-count 1
   ```

3. Create `.env` file:
   ```bash
   cat > test/integration/.env << EOF
   RUN_AWS_INTEGRATION_TESTS=true
   AWS_REGION=us-east-1
   TEST_S3_BUCKET=your-test-bucket-name
   TEST_KINESIS_TRACES_STREAM=test-telemetry-traces
   TEST_KINESIS_METRICS_STREAM=test-telemetry-metrics
   TEST_KINESIS_LOGS_STREAM=test-telemetry-logs
   EOF
   ```

## Running Tests

### Full Integration Test Suite

```bash
cd test/integration
source .env
go test -v -timeout 10m
```

### Specific Test Functions

```bash
# Test only S3 operations
go test -v -run TestS3Operations

# Test only Kinesis operations
go test -v -run TestKinesis

# Test error handling
go test -v -run TestErrorHandling

# Test concurrent operations
go test -v -run TestConcurrentOperations
```

### Using Make Targets

```bash
# Run integration tests (from project root)
make test-integration

# Run with specific environment
ENV=staging make test-integration
```

## Test Structure

### Test Suite Organization

```go
type AWSIntegrationTestSuite struct {
    suite.Suite
    s3Client      *s3.Client       // AWS S3 client
    kinesisClient *kinesis.Client  // AWS Kinesis client
    testBucket    string           // Test S3 bucket name
    testStreams   []string         // Test Kinesis stream names
}
```

### Test Categories

1. **S3 Tests**:
   - Upload/download operations
   - Object metadata verification
   - Error scenarios (non-existent bucket/key)

2. **Kinesis Tests**:
   - Single record puts
   - Batch record puts
   - Stream validation
   - Error scenarios (invalid streams/keys)

3. **Concurrent Tests**:
   - Multiple simultaneous uploads
   - Concurrent Kinesis puts
   - Race condition validation

4. **Error Handling**:
   - AWS service errors
   - Network timeout scenarios
   - Invalid parameter handling

### Test Data

Tests use realistic telemetry data:

```go
type IntegrationTestData struct {
    Traces  []map[string]interface{} // OpenTelemetry traces
    Metrics []map[string]interface{} // Prometheus-style metrics
    Logs    []map[string]interface{} // Structured log entries
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RUN_AWS_INTEGRATION_TESTS` | `false` | Enable integration tests |
| `AWS_REGION` | `us-east-1` | AWS region for resources |
| `TEST_S3_BUCKET` | `playback-test-bucket` | S3 bucket name |
| `TEST_KINESIS_TRACES_STREAM` | `test-telemetry-traces` | Kinesis traces stream |
| `TEST_KINESIS_METRICS_STREAM` | `test-telemetry-metrics` | Kinesis metrics stream |
| `TEST_KINESIS_LOGS_STREAM` | `test-telemetry-logs` | Kinesis logs stream |

### Test Timeouts

- Default test timeout: 10 minutes
- Individual operation timeout: 30 seconds
- Stream creation wait: 5 minutes

## Cleanup

### Automatic Cleanup

```bash
# Clean up all AWS resources
./test/integration/cleanup_aws_resources.sh
```

### Manual Cleanup

```bash
# Delete S3 bucket (and all objects)
aws s3 rb s3://your-test-bucket --force

# Delete Kinesis streams
aws kinesis delete-stream --stream-name test-telemetry-traces
aws kinesis delete-stream --stream-name test-telemetry-metrics
aws kinesis delete-stream --stream-name test-telemetry-logs
```

## Cost Considerations

### AWS Service Costs

- **S3**: Minimal costs for storage and requests
- **Kinesis**: $0.015 per shard hour + $0.014 per million records
- **Data Transfer**: Standard AWS data transfer rates apply

### Cost Optimization

- Tests use minimal shard counts (1 per stream)
- S3 lifecycle policies auto-delete test objects after 1 day
- Cleanup script removes all resources when done

**Estimated cost for full test run: < $0.50**

## Troubleshooting

### Common Issues

1. **AWS Credentials Not Found**:
   ```bash
   aws configure
   # or
   export AWS_ACCESS_KEY_ID=your-key
   export AWS_SECRET_ACCESS_KEY=your-secret
   ```

2. **Permission Denied**:
   - Check IAM policies match requirements above
   - Verify credentials have sufficient permissions

3. **Stream Not Ready**:
   - Kinesis streams take time to become active
   - Wait for `ACTIVE` status before running tests

4. **Bucket Already Exists**:
   - S3 bucket names are globally unique
   - Use timestamp suffix in bucket name

### Debug Mode

```bash
# Enable verbose AWS SDK logging
export VERBOSE_AWS_LOGGING=true
go test -v -run TestName
```

### Logs and Monitoring

- Test logs include AWS operation details
- Failed tests preserve AWS resources for debugging
- CloudWatch logs capture AWS service interactions

## Contributing

When adding new integration tests:

1. Follow existing test patterns and naming
2. Include proper cleanup in test teardown
3. Add cost estimates for new AWS services
4. Update this README with new test categories
5. Ensure tests are idempotent and can run multiple times

## Security Notes

- Test resources use restrictive lifecycle policies
- No production data should be used in tests
- Credentials are never logged or stored in code
- Test buckets use random suffixes to prevent conflicts