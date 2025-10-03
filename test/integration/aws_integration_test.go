package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AWSIntegrationTestSuite runs integration tests against real AWS services
type AWSIntegrationTestSuite struct {
	suite.Suite

	// AWS clients
	s3Client      *s3.Client
	kinesisClient *kinesis.Client

	// Test configuration
	testBucket        string
	testStreamTrace   string
	testStreamMetrics string
	testStreamLogs    string
	testRegion        string

	// Test data
	testData       *IntegrationTestData
	createdObjects []string // Track created objects for cleanup
}

// IntegrationTestData contains test telemetry data
type IntegrationTestData struct {
	Traces  []map[string]interface{} `json:"traces"`
	Metrics []map[string]interface{} `json:"metrics"`
	Logs    []map[string]interface{} `json:"logs"`
}

// SetupSuite initializes the test suite with AWS clients and configuration
func (suite *AWSIntegrationTestSuite) SetupSuite() {
	// Skip integration tests if not explicitly enabled
	if os.Getenv("RUN_AWS_INTEGRATION_TESTS") != "true" {
		suite.T().Skip("AWS integration tests not enabled. Set RUN_AWS_INTEGRATION_TESTS=true to run.")
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	require.NoError(suite.T(), err, "Failed to load AWS config")

	// Initialize AWS clients
	suite.s3Client = s3.NewFromConfig(cfg)
	suite.kinesisClient = kinesis.NewFromConfig(cfg)

	// Test configuration from environment
	suite.testRegion = getEnvOrDefault("AWS_REGION", "us-east-1")
	suite.testBucket = getEnvOrDefault("TEST_S3_BUCKET", "playback-test-bucket")
	suite.testStreamTrace = getEnvOrDefault("TEST_KINESIS_TRACES_STREAM", "test-telemetry-traces")
	suite.testStreamMetrics = getEnvOrDefault("TEST_KINESIS_METRICS_STREAM", "test-telemetry-metrics")
	suite.testStreamLogs = getEnvOrDefault("TEST_KINESIS_LOGS_STREAM", "test-telemetry-logs")

	// Prepare test data
	suite.testData = suite.generateIntegrationTestData()

	// Verify AWS resources exist or create them if needed
	suite.ensureAWSResourcesExist()

	fmt.Printf("🔧 AWS Integration Test Suite initialized\n")
	fmt.Printf("Region: %s\n", suite.testRegion)
	fmt.Printf("S3 Bucket: %s\n", suite.testBucket)
	fmt.Printf("Kinesis Streams: %s, %s, %s\n",
		suite.testStreamTrace, suite.testStreamMetrics, suite.testStreamLogs)
}

// TearDownSuite cleans up resources after all tests
func (suite *AWSIntegrationTestSuite) TearDownSuite() {
	if os.Getenv("RUN_AWS_INTEGRATION_TESTS") != "true" {
		return
	}

	// Clean up created S3 objects
	for _, objectKey := range suite.createdObjects {
		_, err := suite.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: &suite.testBucket,
			Key:    &objectKey,
		})
		if err != nil {
			fmt.Printf("⚠️  Failed to delete S3 object %s: %v\n", objectKey, err)
		}
	}

	fmt.Printf("🧹 AWS Integration Test Suite cleanup completed\n")
}

// TestS3Operations tests S3 upload, download, and listing operations
func (suite *AWSIntegrationTestSuite) TestS3Operations() {
	ctx := context.TODO()
	testKey := fmt.Sprintf("integration-test/traces-%d.json", time.Now().Unix())

	// Test data upload
	testData := map[string]interface{}{
		"test_id":   "s3-integration-test",
		"timestamp": time.Now().Unix(),
		"data":      suite.testData.Traces[:5], // Upload subset
	}

	jsonData, err := json.Marshal(testData)
	require.NoError(suite.T(), err)

	// Upload to S3
	uploader := manager.NewUploader(suite.s3Client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &suite.testBucket,
		Key:         &testKey,
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	})
	require.NoError(suite.T(), err, "Failed to upload to S3")
	suite.createdObjects = append(suite.createdObjects, testKey)

	// Verify object exists
	_, err = suite.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &suite.testBucket,
		Key:    &testKey,
	})
	require.NoError(suite.T(), err, "Object should exist in S3")

	// Download and verify content
	downloader := manager.NewDownloader(suite.s3Client)
	buffer := manager.NewWriteAtBuffer([]byte{})

	_, err = downloader.Download(ctx, buffer, &s3.GetObjectInput{
		Bucket: &suite.testBucket,
		Key:    &testKey,
	})
	require.NoError(suite.T(), err, "Failed to download from S3")

	// Verify downloaded data matches uploaded data
	var downloadedData map[string]interface{}
	err = json.Unmarshal(buffer.Bytes(), &downloadedData)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), testData["test_id"], downloadedData["test_id"])
	assert.NotNil(suite.T(), downloadedData["data"])

	fmt.Printf("✅ S3 operations test passed\n")
}

// TestKinesisTracesStream tests Kinesis operations for traces stream
func (suite *AWSIntegrationTestSuite) TestKinesisTracesStream() {
	suite.testKinesisStream(suite.testStreamTrace, suite.testData.Traces[:3], "traces")
}

// TestKinesisMetricsStream tests Kinesis operations for metrics stream
func (suite *AWSIntegrationTestSuite) TestKinesisMetricsStream() {
	suite.testKinesisStream(suite.testStreamMetrics, suite.testData.Metrics[:3], "metrics")
}

// TestKinesisLogsStream tests Kinesis operations for logs stream
func (suite *AWSIntegrationTestSuite) TestKinesisLogsStream() {
	suite.testKinesisStream(suite.testStreamLogs, suite.testData.Logs[:3], "logs")
}

// testKinesisStream performs Kinesis stream operations test
func (suite *AWSIntegrationTestSuite) testKinesisStream(streamName string, testData []map[string]interface{}, dataType string) {
	ctx := context.TODO()

	// Verify stream exists and is active
	describeOutput, err := suite.kinesisClient.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: &streamName,
	})
	require.NoError(suite.T(), err, "Failed to describe Kinesis stream")
	assert.Equal(suite.T(), types.StreamStatusActive, describeOutput.StreamDescription.StreamStatus)

	// Test single record put
	singleRecord := testData[0]
	recordData, err := json.Marshal(singleRecord)
	require.NoError(suite.T(), err)

	partitionKey := fmt.Sprintf("%s-test-%d", dataType, time.Now().Unix())

	putRecordOutput, err := suite.kinesisClient.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   &streamName,
		Data:         recordData,
		PartitionKey: &partitionKey,
	})
	require.NoError(suite.T(), err, "Failed to put single record")
	assert.NotEmpty(suite.T(), putRecordOutput.SequenceNumber)
	assert.NotEmpty(suite.T(), putRecordOutput.ShardId)

	// Test batch put records
	var records []types.PutRecordsRequestEntry
	for i, record := range testData {
		data, err := json.Marshal(record)
		require.NoError(suite.T(), err)

		partitionKey := fmt.Sprintf("%s-batch-%d-%d", dataType, time.Now().Unix(), i)
		records = append(records, types.PutRecordsRequestEntry{
			Data:         data,
			PartitionKey: &partitionKey,
		})
	}

	putRecordsOutput, err := suite.kinesisClient.PutRecords(ctx, &kinesis.PutRecordsInput{
		StreamName: &streamName,
		Records:    records,
	})
	require.NoError(suite.T(), err, "Failed to put batch records")
	assert.Equal(suite.T(), int32(0), putRecordsOutput.FailedRecordCount)
	assert.Len(suite.T(), putRecordsOutput.Records, len(testData))

	// Verify all batch records succeeded
	for i, recordResult := range putRecordsOutput.Records {
		assert.NotEmpty(suite.T(), recordResult.SequenceNumber, "Record %d should have sequence number", i)
		assert.NotEmpty(suite.T(), recordResult.ShardId, "Record %d should have shard ID", i)
		assert.Nil(suite.T(), recordResult.ErrorCode, "Record %d should not have error", i)
	}

	fmt.Printf("✅ Kinesis %s stream test passed\n", dataType)
}

// TestKinesisErrorHandling tests error scenarios
func (suite *AWSIntegrationTestSuite) TestKinesisErrorHandling() {
	ctx := context.TODO()

	// Test with non-existent stream
	nonExistentStream := "non-existent-stream-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := suite.kinesisClient.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   &nonExistentStream,
		Data:         []byte("test data"),
		PartitionKey: aws.String("test-key"),
	})
	require.Error(suite.T(), err, "Should fail with non-existent stream")

	// Test with invalid partition key (too long)
	longPartitionKey := make([]byte, 300) // Max is 256 bytes
	for i := range longPartitionKey {
		longPartitionKey[i] = 'a'
	}

	_, err = suite.kinesisClient.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   &suite.testStreamTrace,
		Data:         []byte("test data"),
		PartitionKey: aws.String(string(longPartitionKey)),
	})
	require.Error(suite.T(), err, "Should fail with invalid partition key")

	fmt.Printf("✅ Kinesis error handling test passed\n")
}

// TestS3ErrorHandling tests S3 error scenarios
func (suite *AWSIntegrationTestSuite) TestS3ErrorHandling() {
	ctx := context.TODO()

	// Test with non-existent bucket
	nonExistentBucket := "non-existent-bucket-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := suite.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &nonExistentBucket,
		Key:    aws.String("test-key"),
		Body:   bytes.NewReader([]byte("test data")),
	})
	require.Error(suite.T(), err, "Should fail with non-existent bucket")

	// Test download from non-existent key
	nonExistentKey := "non-existent-key-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err = suite.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &suite.testBucket,
		Key:    &nonExistentKey,
	})
	require.Error(suite.T(), err, "Should fail with non-existent key")

	fmt.Printf("✅ S3 error handling test passed\n")
}

// TestConcurrentOperations tests concurrent AWS operations
func (suite *AWSIntegrationTestSuite) TestConcurrentOperations() {
	ctx := context.TODO()

	// Test concurrent S3 uploads
	const concurrentUploads = 5
	uploadResults := make(chan error, concurrentUploads)

	for i := 0; i < concurrentUploads; i++ {
		go func(index int) {
			testKey := fmt.Sprintf("concurrent-test/upload-%d-%d.json", index, time.Now().Unix())
			testData := map[string]interface{}{
				"index":     index,
				"timestamp": time.Now().Unix(),
				"data":      suite.testData.Traces[index%len(suite.testData.Traces)],
			}

			jsonData, err := json.Marshal(testData)
			if err != nil {
				uploadResults <- err
				return
			}

			uploader := manager.NewUploader(suite.s3Client)
			_, err = uploader.Upload(ctx, &s3.PutObjectInput{
				Bucket: &suite.testBucket,
				Key:    &testKey,
				Body:   bytes.NewReader(jsonData),
			})

			if err == nil {
				suite.createdObjects = append(suite.createdObjects, testKey)
			}

			uploadResults <- err
		}(i)
	}

	// Wait for all uploads to complete
	for i := 0; i < concurrentUploads; i++ {
		err := <-uploadResults
		require.NoError(suite.T(), err, "Concurrent upload %d should succeed", i)
	}

	// Test concurrent Kinesis puts
	const concurrentPuts = 5
	putResults := make(chan error, concurrentPuts)

	for i := 0; i < concurrentPuts; i++ {
		go func(index int) {
			recordData, err := json.Marshal(suite.testData.Traces[index%len(suite.testData.Traces)])
			if err != nil {
				putResults <- err
				return
			}

			partitionKey := fmt.Sprintf("concurrent-test-%d-%d", index, time.Now().Unix())
			_, err = suite.kinesisClient.PutRecord(ctx, &kinesis.PutRecordInput{
				StreamName:   &suite.testStreamTrace,
				Data:         recordData,
				PartitionKey: &partitionKey,
			})

			putResults <- err
		}(i)
	}

	// Wait for all puts to complete
	for i := 0; i < concurrentPuts; i++ {
		err := <-putResults
		require.NoError(suite.T(), err, "Concurrent put %d should succeed", i)
	}

	fmt.Printf("✅ Concurrent operations test passed\n")
}

// Helper methods

// ensureAWSResourcesExist verifies that required AWS resources exist
func (suite *AWSIntegrationTestSuite) ensureAWSResourcesExist() {
	ctx := context.TODO()

	// Check S3 bucket
	_, err := suite.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &suite.testBucket,
	})
	if err != nil {
		suite.T().Fatalf("Test S3 bucket '%s' does not exist or is not accessible: %v", suite.testBucket, err)
	}

	// Check Kinesis streams
	streams := []string{suite.testStreamTrace, suite.testStreamMetrics, suite.testStreamLogs}
	for _, streamName := range streams {
		_, err := suite.kinesisClient.DescribeStream(ctx, &kinesis.DescribeStreamInput{
			StreamName: &streamName,
		})
		if err != nil {
			suite.T().Fatalf("Test Kinesis stream '%s' does not exist or is not accessible: %v", streamName, err)
		}
	}
}

// generateIntegrationTestData creates realistic test data for integration testing
func (suite *AWSIntegrationTestSuite) generateIntegrationTestData() *IntegrationTestData {
	now := time.Now()

	// Generate trace data
	traces := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		traces[i] = map[string]interface{}{
			"traceId":   fmt.Sprintf("trace-%d-%d", i, now.Unix()),
			"spanId":    fmt.Sprintf("span-%d", i),
			"service":   "integration-test-service",
			"operation": fmt.Sprintf("test-operation-%d", i%3),
			"startTime": now.Add(-time.Duration(i) * time.Second).Unix(),
			"duration":  (100 + i*10), // milliseconds
			"status": map[string]interface{}{
				"code":    0,
				"message": "OK",
			},
		}
	}

	// Generate metrics data
	metrics := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		metrics[i] = map[string]interface{}{
			"name":      fmt.Sprintf("integration_test_metric_%d", i%5),
			"value":     float64(100 + i*5),
			"timestamp": now.Add(-time.Duration(i) * time.Minute).Unix(),
			"labels": map[string]string{
				"environment": "integration-test",
				"service":     "test-service",
				"index":       strconv.Itoa(i),
			},
		}
	}

	// Generate logs data
	logs := make([]map[string]interface{}, 10)
	logLevels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	for i := 0; i < 10; i++ {
		logs[i] = map[string]interface{}{
			"timestamp": now.Add(-time.Duration(i) * time.Second).Unix(),
			"level":     logLevels[i%len(logLevels)],
			"message":   fmt.Sprintf("Integration test log message %d", i),
			"service":   "integration-test-service",
			"attributes": map[string]interface{}{
				"thread_id":  i % 5,
				"request_id": fmt.Sprintf("req-%d", i),
			},
		}
	}

	return &IntegrationTestData{
		Traces:  traces,
		Metrics: metrics,
		Logs:    logs,
	}
}

// Utility function
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestAWSIntegrationSuite runs the integration test suite
func TestAWSIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AWSIntegrationTestSuite))
}
