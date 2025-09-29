#!/bin/bash

# AWS Integration Test Resource Cleanup Script
# Removes AWS resources created for integration testing

set -e

# Load configuration from environment file if it exists
if [ -f "test/integration/.env" ]; then
    source test/integration/.env
    echo "📖 Loaded configuration from test/integration/.env"
else
    echo "⚠️  Environment file not found. Using default values."
fi

# Configuration with fallbacks
AWS_REGION=${AWS_REGION:-us-east-1}
TEST_S3_BUCKET=${TEST_S3_BUCKET:-playback-test-bucket}
TEST_KINESIS_TRACES_STREAM=${TEST_KINESIS_TRACES_STREAM:-test-telemetry-traces}
TEST_KINESIS_METRICS_STREAM=${TEST_KINESIS_METRICS_STREAM:-test-telemetry-metrics}
TEST_KINESIS_LOGS_STREAM=${TEST_KINESIS_LOGS_STREAM:-test-telemetry-logs}

echo "🧹 Cleaning up AWS resources for integration testing..."
echo "Region: $AWS_REGION"
echo "S3 Bucket: $TEST_S3_BUCKET"
echo "Kinesis Streams: $TEST_KINESIS_TRACES_STREAM, $TEST_KINESIS_METRICS_STREAM, $TEST_KINESIS_LOGS_STREAM"
echo

# Confirmation prompt
read -p "⚠️  Are you sure you want to delete these AWS resources? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cleanup cancelled."
    exit 1
fi

# Check AWS CLI is available
if ! command -v aws &> /dev/null; then
    echo "❌ AWS CLI not found. Please install AWS CLI."
    exit 1
fi

# Verify AWS credentials
echo "🔐 Verifying AWS credentials..."
aws sts get-caller-identity > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "❌ AWS credentials not configured. Please run 'aws configure' first."
    exit 1
fi
echo "✅ AWS credentials verified"

# Function to delete Kinesis stream
delete_kinesis_stream() {
    local stream_name=$1
    echo "🗑️  Deleting Kinesis stream: $stream_name"

    if aws kinesis describe-stream --stream-name "$stream_name" --region "$AWS_REGION" >/dev/null 2>&1; then
        aws kinesis delete-stream \
            --stream-name "$stream_name" \
            --region "$AWS_REGION"

        echo "⏳ Waiting for stream deletion: $stream_name"

        # Wait for stream to be deleted (custom wait since there's no built-in wait for deletion)
        local max_attempts=30
        local attempt=0

        while [ $attempt -lt $max_attempts ]; do
            if ! aws kinesis describe-stream --stream-name "$stream_name" --region "$AWS_REGION" >/dev/null 2>&1; then
                echo "✅ Kinesis stream deleted: $stream_name"
                return 0
            fi

            echo "   Still deleting... (attempt $((attempt + 1))/$max_attempts)"
            sleep 10
            attempt=$((attempt + 1))
        done

        echo "⚠️  Stream deletion timed out, but deletion may still be in progress: $stream_name"
    else
        echo "ℹ️  Kinesis stream does not exist (already deleted?): $stream_name"
    fi
}

# Delete Kinesis streams
delete_kinesis_stream "$TEST_KINESIS_TRACES_STREAM"
delete_kinesis_stream "$TEST_KINESIS_METRICS_STREAM"
delete_kinesis_stream "$TEST_KINESIS_LOGS_STREAM"

# Delete S3 bucket and all its contents
echo "📦 Deleting S3 bucket: $TEST_S3_BUCKET"
if aws s3api head-bucket --bucket "$TEST_S3_BUCKET" --region "$AWS_REGION" 2>/dev/null; then
    # Delete all objects in the bucket first
    echo "🗑️  Deleting all objects in bucket..."
    aws s3 rm "s3://$TEST_S3_BUCKET" --recursive --region "$AWS_REGION"

    # Delete the bucket
    aws s3api delete-bucket \
        --bucket "$TEST_S3_BUCKET" \
        --region "$AWS_REGION"

    echo "✅ S3 bucket deleted: $TEST_S3_BUCKET"
else
    echo "ℹ️  S3 bucket does not exist (already deleted?): $TEST_S3_BUCKET"
fi

# Clean up environment file
if [ -f "test/integration/.env" ]; then
    echo "📝 Removing integration test environment file..."
    rm test/integration/.env
    echo "✅ Environment file removed"
fi

echo
echo "🎉 AWS integration test resources cleanup completed!"
echo
echo "Resources that were deleted:"
echo "  S3 Bucket: $TEST_S3_BUCKET"
echo "  Kinesis Streams:"
echo "    - $TEST_KINESIS_TRACES_STREAM"
echo "    - $TEST_KINESIS_METRICS_STREAM"
echo "    - $TEST_KINESIS_LOGS_STREAM"
echo "  Environment file: test/integration/.env"
echo
echo "ℹ️  Note: Kinesis stream deletions may take a few minutes to complete fully."