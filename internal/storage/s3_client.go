package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

type S3Config struct {
	Region          string
	EndpointURL     string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	ForcePathStyle  bool
}

// NewS3Client creates a new S3 client with the provided configuration
func NewS3Client(cfg *S3Config) (*s3.Client, error) {
	logger.Info("Initializing S3 client",
		zap.String("region", cfg.Region),
		zap.String("endpoint", cfg.EndpointURL),
		zap.String("bucket", cfg.Bucket))

	// Create AWS config
	var awsCfg aws.Config
	var err error

	// If we have custom credentials, use them
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					cfg.AccessKeyID,
					cfg.SecretAccessKey,
					"",
				),
			),
		)
	} else {
		// Use default credential chain
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	options := []func(*s3.Options){}

	// If we have a custom endpoint (for LocalStack), use it
	if cfg.EndpointURL != "" {
		options = append(options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		})
	}

	// Force path-style addressing for LocalStack compatibility
	if cfg.ForcePathStyle {
		options = append(options, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, options...)

	logger.Info("S3 client initialized successfully")
	return client, nil
}