// Package s3 defines configuration for AWS S3 (Simple Storage Service) object storage.
//
// Amazon S3 is an object storage service offering industry-leading scalability, data availability,
// security, and performance. This package provides configuration for S3 client connections including
// bucket management, authentication, server-side encryption, and local development support.
//
// Official AWS S3 Documentation: https://docs.aws.amazon.com/s3/
// AWS S3 Go SDK v2: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
//
// # Overview
//
// S3 configuration enables:
//
//   - Object storage for telemetry data (traces, metrics, logs)
//   - Configurable bucket and path prefix organization
//   - Server-side encryption (SSE-S3, SSE-KMS, SSE-C)
//   - AWS authentication with IAM roles or credentials
//   - LocalStack integration for local development and testing
//   - Custom endpoint support for S3-compatible services (MinIO, Ceph)
//
// # Configuration Keys
//
// All settings use the S3_ prefix and support environment variable overrides:
//
//	S3_REGION             - AWS region (default: us-east-1)
//	S3_BUCKET             - S3 bucket name (required)
//	S3_ENDPOINT_URL       - Custom endpoint for LocalStack/MinIO (optional)
//	S3_ACCESS_KEY_ID      - AWS access key (optional, prefer IAM roles)
//	S3_SECRET_ACCESS_KEY  - AWS secret key (optional, prefer IAM roles)
//	S3_SESSION_TOKEN      - AWS session token for temporary credentials (optional)
//	S3_PATH_PREFIX        - Object key prefix for organization (optional)
//	S3_FORCE_PATH_STYLE   - Use path-style URLs (default: false, required for LocalStack)
//	S3_ENABLE_SSE         - Enable server-side encryption (default: true)
//
// # Server-Side Encryption
//
// S3 supports multiple server-side encryption options to protect data at rest:
//
// SSE-S3 (Server-Side Encryption with Amazon S3-Managed Keys):
//   - Encryption keys managed by AWS
//   - AES-256 encryption
//   - No additional cost
//   - Enabled by setting S3_ENABLE_SSE=true (default)
//   - AWS manages key rotation automatically
//   - Learn more: https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingServerSideEncryption.html
//
// SSE-KMS (Server-Side Encryption with AWS Key Management Service):
//   - Encryption keys managed in AWS KMS
//   - Customer-managed keys (CMK) or AWS-managed keys
//   - Audit trail of key usage via CloudTrail
//   - Additional cost per key and API call
//   - Requires kms:Decrypt and kms:GenerateDataKey permissions
//   - Learn more: https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingKMSEncryption.html
//
// SSE-C (Server-Side Encryption with Customer-Provided Keys):
//   - Customer provides encryption keys with each request
//   - Customer manages key rotation
//   - No AWS key storage
//   - Must provide key in every GET/PUT request
//   - Learn more: https://docs.aws.amazon.com/AmazonS3/latest/userguide/ServerSideEncryptionCustomerKeys.html
//
// Encryption best practices:
//
//	# Production: Enable SSE-S3 (default, simplest)
//	S3_ENABLE_SSE=true
//
//	# Production with compliance requirements: Use SSE-KMS
//	S3_ENABLE_SSE=true
//	# Configure KMS key ARN in bucket policy
//
//	# Maximum security: Use SSE-C with application-managed keys
//	S3_ENABLE_SSE=false
//	# Provide encryption keys in SDK calls
//
// # Authentication
//
// S3 supports multiple authentication methods, evaluated in this order:
//
// 1. Environment Variables (explicit configuration):
//
//	S3_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
//	S3_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
//	S3_SESSION_TOKEN=AQoDYXdzEJr...  # For temporary credentials
//
// 2. IAM Instance Role (recommended for EC2/ECS/Lambda):
//   - No explicit credentials needed
//   - Automatic credential rotation
//   - Limited by instance role policies
//   - Best practice for production deployments
//
// 3. AWS Credentials File (~/.aws/credentials):
//
//	[default]
//	aws_access_key_id = AKIAIOSFODNN7EXAMPLE
//	aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
//
// 4. Environment Variables (AWS SDK default):
//
//	AWS_ACCESS_KEY_ID=...
//	AWS_SECRET_ACCESS_KEY=...
//	AWS_SESSION_TOKEN=...
//
// Production recommendation: Use IAM roles wherever possible to avoid managing long-lived credentials.
//
// IAM permissions required:
//
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [{
//	        "Effect": "Allow",
//	        "Action": [
//	            "s3:PutObject",
//	            "s3:GetObject",
//	            "s3:DeleteObject",
//	            "s3:ListBucket"
//	        ],
//	        "Resource": [
//	            "arn:aws:s3:::my-bucket",
//	            "arn:aws:s3:::my-bucket/*"
//	        ]
//	    }]
//	}
//
// # Bucket Organization
//
// Use PathPrefix to organize objects within a bucket:
//
//	S3_BUCKET=telemetry-data
//	S3_PATH_PREFIX=prod/traces/
//
// This creates objects with keys like: prod/traces/2025/01/03/trace-12345.json
//
// Common organization patterns:
//
// By environment:
//
//	S3_PATH_PREFIX=prod/          # Production data
//	S3_PATH_PREFIX=staging/       # Staging data
//	S3_PATH_PREFIX=dev/           # Development data
//
// By data type:
//
//	S3_PATH_PREFIX=traces/        # Distributed traces
//	S3_PATH_PREFIX=metrics/       # Time-series metrics
//	S3_PATH_PREFIX=logs/          # Application logs
//
// By date partitioning (recommended for large-scale):
//
//	S3_PATH_PREFIX=data/year=2025/month=01/day=03/
//	# Enables efficient queries with Athena/Glue
//
// # LocalStack Integration
//
// LocalStack provides a local AWS cloud stack for development and testing.
//
// Docker Compose setup:
//
//	services:
//	  localstack:
//	    image: localstack/localstack:latest
//	    ports:
//	      - "4566:4566"
//	    environment:
//	      - SERVICES=s3
//	      - DEBUG=1
//	      - DATA_DIR=/tmp/localstack/data
//	    volumes:
//	      - "./localstack:/tmp/localstack"
//
// Configuration for LocalStack:
//
//	S3_ENDPOINT_URL=http://localhost:4566
//	S3_REGION=us-east-1
//	S3_BUCKET=test-bucket
//	S3_ACCESS_KEY_ID=test
//	S3_SECRET_ACCESS_KEY=test
//	S3_FORCE_PATH_STYLE=true    # REQUIRED for LocalStack
//
// Create bucket in LocalStack:
//
//	aws --endpoint-url=http://localhost:4566 s3 mb s3://test-bucket
//
// ForcePathStyle explanation:
//   - Virtual-hosted style: https://bucket.s3.amazonaws.com/key
//   - Path style: https://s3.amazonaws.com/bucket/key
//   - LocalStack requires path style for DNS resolution
//   - AWS S3 virtual-hosted style is preferred in production
//
// Learn more: https://docs.localstack.cloud/user-guide/aws/s3/
//
// # S3-Compatible Services
//
// This configuration also works with S3-compatible object storage services:
//
// MinIO (open-source S3-compatible storage):
//
//	S3_ENDPOINT_URL=http://localhost:9000
//	S3_ACCESS_KEY_ID=minioadmin
//	S3_SECRET_ACCESS_KEY=minioadmin
//	S3_FORCE_PATH_STYLE=true
//	S3_BUCKET=telemetry
//
// Ceph RADOS Gateway:
//
//	S3_ENDPOINT_URL=http://ceph-gateway:7480
//	S3_ACCESS_KEY_ID=access_key
//	S3_SECRET_ACCESS_KEY=secret_key
//	S3_FORCE_PATH_STYLE=true
//
// DigitalOcean Spaces:
//
//	S3_ENDPOINT_URL=https://nyc3.digitaloceanspaces.com
//	S3_REGION=nyc3
//	S3_ACCESS_KEY_ID=spaces_key
//	S3_SECRET_ACCESS_KEY=spaces_secret
//
// # Example Usage
//
//	// Load S3 configuration
//	cfg, err := s3.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create AWS config
//	awsCfg, err := config.LoadDefaultConfig(context.Background(),
//	    config.WithRegion(string(cfg.Region)),
//	    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
//	        cfg.AccessKeyID,
//	        cfg.SecretAccessKey,
//	        cfg.SessionToken,
//	    )),
//	)
//
//	// Create S3 client with custom endpoint
//	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
//	    if cfg.EndpointURL != "" {
//	        o.BaseEndpoint = aws.String(cfg.EndpointURL)
//	        o.UsePathStyle = cfg.ForcePathStyle
//	    }
//	})
//
//	// Upload object with SSE
//	key := filepath.Join(cfg.PathPrefix, "trace-12345.json")
//	_, err = s3Client.PutObject(context.Background(), &s3.PutObjectInput{
//	    Bucket: aws.String(cfg.Bucket),
//	    Key:    aws.String(key),
//	    Body:   bytes.NewReader(data),
//	    ServerSideEncryption: types.ServerSideEncryptionAes256, // If cfg.EnableSSE
//	})
//
//	// Download object
//	result, err := s3Client.GetObject(context.Background(), &s3.GetObjectInput{
//	    Bucket: aws.String(cfg.Bucket),
//	    Key:    aws.String(key),
//	})
//	defer result.Body.Close()
//	data, _ := io.ReadAll(result.Body)
//
// # Storage Classes and Lifecycle
//
// S3 offers multiple storage classes with different cost/access tradeoffs:
//
//	STANDARD        - Frequently accessed data (default)
//	STANDARD_IA     - Infrequently accessed (cheaper storage, retrieval fee)
//	GLACIER         - Archive storage (very cheap, slow retrieval)
//	GLACIER_DEEP    - Long-term archive (cheapest, slowest retrieval)
//
// Configure lifecycle policies in AWS Console or via SDK:
//
//	# Transition traces to IA after 30 days, Glacier after 90 days
//	{
//	    "Rules": [{
//	        "Id": "trace-lifecycle",
//	        "Prefix": "traces/",
//	        "Status": "Enabled",
//	        "Transitions": [
//	            {"Days": 30, "StorageClass": "STANDARD_IA"},
//	            {"Days": 90, "StorageClass": "GLACIER"}
//	        ],
//	        "Expiration": {"Days": 365}
//	    }]
//	}
//
// Learn more: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lifecycle-mgmt.html
//
// # Best Practices
//
// Production configuration:
//
//	S3_REGION=us-east-1              # Choose region near your services
//	S3_BUCKET=prod-telemetry         # Dedicated bucket per environment
//	S3_PATH_PREFIX=traces/           # Organize by data type
//	S3_ENABLE_SSE=true               # Always encrypt at rest
//	# Use IAM role instead of access keys
//
// Development configuration:
//
//	S3_ENDPOINT_URL=http://localhost:4566
//	S3_REGION=us-east-1
//	S3_BUCKET=dev-telemetry
//	S3_ACCESS_KEY_ID=test
//	S3_SECRET_ACCESS_KEY=test
//	S3_FORCE_PATH_STYLE=true
//	S3_ENABLE_SSE=false              # Optional for local development
//
// Testing configuration:
//
//	S3_ENDPOINT_URL=http://localstack:4566
//	S3_BUCKET=test-bucket
//	S3_ACCESS_KEY_ID=test
//	S3_SECRET_ACCESS_KEY=test
//	S3_FORCE_PATH_STYLE=true
//
// Security recommendations:
//   - Enable SSE encryption in production (S3_ENABLE_SSE=true)
//   - Use IAM roles over access keys when running on AWS
//   - Enable S3 bucket versioning for data protection
//   - Configure bucket policies to restrict access
//   - Enable S3 access logging for audit trails
//   - Use VPC endpoints to keep traffic within AWS network
//   - Enable MFA delete for critical buckets
//
// Performance optimization:
//   - Use multipart upload for objects >100MB
//   - Enable Transfer Acceleration for global access
//   - Use CloudFront for frequently accessed objects
//   - Parallelize uploads with multiple workers
//   - Use S3 Select to retrieve subset of object data
//
// Cost optimization:
//   - Use lifecycle policies to transition old data to cheaper storage
//   - Enable S3 Intelligent-Tiering for automatic optimization
//   - Delete old data with expiration policies
//   - Use S3 Storage Lens to analyze usage patterns
//   - Compress data before upload (especially logs/traces)
//
// # Troubleshooting
//
// Access denied errors:
//
//	Error: "Access Denied" when uploading
//	Check: IAM permissions include s3:PutObject for bucket
//	Check: Bucket policy allows your IAM role/user
//	Check: Bucket is not in different AWS account
//
// Endpoint connection errors:
//
//	Error: "no such host" or connection refused
//	Fix: Verify S3_ENDPOINT_URL is correct (http://localhost:4566 for LocalStack)
//	Fix: Ensure LocalStack/MinIO container is running
//	Fix: Check network connectivity and firewall rules
//
// Path style errors:
//
//	Error: "PermanentRedirect" or "bucket not found"
//	Fix: Set S3_FORCE_PATH_STYLE=true for LocalStack/MinIO
//	Fix: Ensure bucket name is DNS-compatible (lowercase, no underscores)
//
// Encryption errors:
//
//	Error: "Access Denied" with SSE-KMS
//	Fix: Ensure IAM role has kms:Decrypt and kms:GenerateDataKey permissions
//	Fix: KMS key policy allows your IAM principal
//
// Region errors:
//
//	Error: "AuthorizationHeaderMalformed" or region mismatch
//	Fix: Set S3_REGION to match bucket region
//	Fix: Check bucket location with: aws s3api get-bucket-location --bucket <name>
//
// # Cross-References
//
// Related packages:
//   - [base.AWSRegion] - AWS region type definitions
//   - [base.Validator] - Validation framework
//   - [kinesis] - AWS Kinesis streaming configuration
//   - [dlq] - Dead letter queue configuration
//
// AWS documentation:
//   - S3 Developer Guide: https://docs.aws.amazon.com/s3/
//   - S3 API Reference: https://docs.aws.amazon.com/AmazonS3/latest/API/
//   - Go SDK v2: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
//   - IAM Policies: https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html
//   - LocalStack S3: https://docs.localstack.cloud/user-guide/aws/s3/
//
// # Files in This Package
//
// constants.go:
//   - S3_PREFIX for environment variable namespacing
//   - Default values (region, force path style, SSE enabled)
//   - Configuration constants
//
// section.go:
//   - [Config] struct with S3 parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package s3
