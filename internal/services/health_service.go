package services

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Health check timeouts and limits
	DefaultHealthTimeout = 10 * time.Second
	MaxConcurrentChecks  = 5
	MinConcurrentChecks  = 0

	// Memory conversion constants
	BytesPerMegabyte = 1024 * 1024

	// Channel buffer sizes
	HealthResultChannelBuffer = 5

	// Dependency check iteration limits
	MaxStreamCheckCount = 1 // Only check first stream for basic connectivity

	// Health check field keys
	HealthFieldDependencies = "dependencies"
	HealthFieldSystem      = "system"
	HealthFieldMode        = "mode"
	HealthFieldAppVersion  = "app_version"
	HealthFieldRuntime     = "runtime_version"
	HealthFieldProtocols   = "protocols"
	HealthFieldTimestamp   = "timestamp"

	// System metrics field keys
	SystemFieldGoroutines = "goroutines"
	SystemFieldMemoryMB   = "memory_mb"
	SystemFieldAlloc      = "alloc"
	SystemFieldSys        = "sys"
	SystemFieldNumGC      = "num_gc"

	// Error messages
	ErrClickHouseHostNotConfigured = "ClickHouse host not configured"
	ErrNoKinesisStreamsConfigured  = "no Kinesis streams configured"
	ErrHealthCheckPanic           = "health check panic"
	ErrHealthCheckTimeout         = "health check timeout"
)

// HealthService provides comprehensive health checking capabilities
type HealthService struct {
	config         *config.ConsolidatedConfig
	clickHouseClient interface{
		CheckConnectionPoolHealth(ctx context.Context) error
		GetConnectionPoolStats(ctx context.Context) (*ConnectionPoolStats, error)
	}
}

// ConnectionPoolStats represents connection pool statistics (imported from storage package)
type ConnectionPoolStats struct {
	MaxOpenConnections     int     `json:"max_open_connections"`
	OpenConnections        int     `json:"open_connections"`
	InUseConnections       int     `json:"in_use_connections"`
	IdleConnections        int     `json:"idle_connections"`
	UtilizationPercent     float64 `json:"utilization_percent"`
	HealthStatus           string  `json:"health_status"`
	ConnectionsWaiting     int64   `json:"connections_waiting"`
	MaxIdleConnections     int     `json:"max_idle_connections"`
	ConnectionMaxLifetime  string  `json:"connection_max_lifetime"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	DependencyName string
	Status         gin.H
}

// HealthResponse represents the complete health check response
type HealthResponse struct {
	OverallStatus    string            `json:"status"`
	Mode             string            `json:"mode"`
	AppVersion       string            `json:"app_version"`
	RuntimeVersion   string            `json:"runtime_version"`
	Protocols        []string          `json:"protocols"`
	Timestamp        string            `json:"timestamp"`
	Dependencies     map[string]gin.H  `json:"dependencies"`
	System           gin.H             `json:"system"`
}

// NewHealthService creates a new health service
func NewHealthService(cfg *config.ConsolidatedConfig) *HealthService {
	return &HealthService{
		config: cfg,
	}
}

// NewHealthServiceWithClickHouse creates a new health service with ClickHouse client for connection pool monitoring
func NewHealthServiceWithClickHouse(cfg *config.ConsolidatedConfig, clickHouseClient interface{
	CheckConnectionPoolHealth(ctx context.Context) error
	GetConnectionPoolStats(ctx context.Context) (*ConnectionPoolStats, error)
}) *HealthService {
	return &HealthService{
		config:           cfg,
		clickHouseClient: clickHouseClient,
	}
}

// PerformHealthCheck conducts a comprehensive health check of all system dependencies
func (hs *HealthService) PerformHealthCheck(ctx context.Context) (*HealthResponse, int) {
	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, DefaultHealthTimeout)
	defer cancel()

	// Runtime information
	runtimeVersion := getRuntimeVersion()
	appVersion := hs.config.App.Version
	if appVersion == "" {
		appVersion = constants.ErrorUnknownVersion
	}

	// Initialize response
	response := &HealthResponse{
		OverallStatus:    constants.HealthStatusOK,
		Mode:             hs.config.Network.HTTP.Mode,
		AppVersion:       appVersion,
		RuntimeVersion:   runtimeVersion,
		Protocols:        []string{constants.ProtocolHTTPJSON, constants.ProtocolGRPCOTLP},
		Timestamp:        time.Now().Format(constants.StandardTimeFormat),
		Dependencies:     make(map[string]gin.H),
		System: gin.H{
			SystemFieldGoroutines: runtime.NumGoroutine(),
			SystemFieldMemoryMB:   getMemoryUsageMB(),
		},
	}

	// Channel to collect health check results
	resultsChan := make(chan HealthCheckResult, HealthResultChannelBuffer)
	activeChecks := MinConcurrentChecks

	// Launch ClickHouse health check
	if hs.config.Data.ClickHouse.Host != "" {
		activeChecks++
		go hs.checkClickHouseHealth(checkCtx, resultsChan)
	} else {
		response.Dependencies[constants.HealthDependencyDatabase] = gin.H{
			constants.HealthFieldStatus: constants.HealthStatusNotConfigured,
		}
	}

	// Launch Kinesis health check
	if hs.config.Data.Kinesis.TracesStream != "" || hs.config.Data.Kinesis.MetricsStream != "" || hs.config.Data.Kinesis.LogsStream != "" {
		activeChecks++
		go hs.checkKinesisHealth(checkCtx, resultsChan)
	} else {
		response.Dependencies[constants.HealthDependencyKinesis] = gin.H{
			constants.HealthFieldStatus: constants.HealthStatusNotConfigured,
		}
	}

	// Launch connection pool health check if ClickHouse client is available
	if hs.clickHouseClient != nil {
		activeChecks++
		go hs.checkConnectionPoolHealth(checkCtx, resultsChan)
	} else {
		response.Dependencies[constants.HealthDependencyConnectionPool] = gin.H{
			constants.HealthFieldStatus: constants.HealthStatusNotConfigured,
		}
	}

	// Collect results from async health checks
	hs.collectHealthResults(checkCtx, resultsChan, activeChecks, response)

	// Determine HTTP status code
	statusCode := constants.StatusOK
	if response.OverallStatus == constants.HealthStatusUnhealthy {
		statusCode = constants.StatusServiceUnavailable
	}

	return response, int(statusCode)
}

// checkClickHouseHealth performs ClickHouse connectivity check
func (hs *HealthService) checkClickHouseHealth(ctx context.Context, resultsChan chan<- HealthCheckResult) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ClickHouse health check panic recovered", zap.Any("panic", r))
			resultsChan <- HealthCheckResult{
				DependencyName: constants.HealthDependencyDatabase,
				Status: gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckPanic,
				},
			}
		}
	}()

	if err := hs.performClickHouseCheck(ctx); err != nil {
		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyDatabase,
			Status: gin.H{
				constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
				constants.HealthFieldError:  err.Error(),
			},
		}
	} else {
		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyDatabase,
			Status: gin.H{
				constants.HealthFieldStatus: constants.HealthStatusHealthy,
			},
		}
	}
}

// performClickHouseCheck executes the actual ClickHouse connectivity test
func (hs *HealthService) performClickHouseCheck(ctx context.Context) error {
	if hs.config.Data.ClickHouse.Host == "" {
		return errors.New(ErrClickHouseHostNotConfigured)
	}

	// Use native ClickHouse driver with TCP protocol
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{hs.config.Data.ClickHouse.Host},
		Auth: clickhouse.Auth{
			Database: hs.config.Data.ClickHouse.Database,
			Username: hs.config.Data.ClickHouse.Username,
			Password: hs.config.Data.ClickHouse.Password,
		},
		DialTimeout: constants.HealthCheckTimeout,
	})
	if err != nil {
		return fmt.Errorf("ClickHouse connection failed: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close ClickHouse connection", zap.Error(closeErr))
		}
	}()

	// Ping to verify connectivity
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ClickHouse ping failed: %w", err)
	}

	return nil
}

// checkKinesisHealth performs Kinesis connectivity check
func (hs *HealthService) checkKinesisHealth(ctx context.Context, resultsChan chan<- HealthCheckResult) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Kinesis health check panic recovered", zap.Any("panic", r))
			resultsChan <- HealthCheckResult{
				DependencyName: constants.HealthDependencyKinesis,
				Status: gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckPanic,
				},
			}
		}
	}()

	if err := hs.performKinesisCheck(ctx); err != nil {
		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyKinesis,
			Status: gin.H{
				constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
				constants.HealthFieldError:  err.Error(),
			},
		}
	} else {
		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyKinesis,
			Status: gin.H{
				constants.HealthFieldStatus: constants.HealthStatusHealthy,
			},
		}
	}
}

// performKinesisCheck executes the actual Kinesis connectivity test
func (hs *HealthService) performKinesisCheck(ctx context.Context) error {
	if hs.config.Data.Kinesis.TracesStream == "" && hs.config.Data.Kinesis.MetricsStream == "" && hs.config.Data.Kinesis.LogsStream == "" {
		return errors.New(ErrNoKinesisStreamsConfigured)
	}

	// Create AWS session and Kinesis client
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(hs.config.Data.Kinesis.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	kinesisClient := kinesis.NewFromConfig(awsCfg)

	// Check if at least one stream exists and is active
	streamCount := MinConcurrentChecks
	// Check each configured stream
	streams := []string{}
	if hs.config.Data.Kinesis.TracesStream != "" {
		streams = append(streams, hs.config.Data.Kinesis.TracesStream)
	}
	if hs.config.Data.Kinesis.MetricsStream != "" {
		streams = append(streams, hs.config.Data.Kinesis.MetricsStream)
	}
	if hs.config.Data.Kinesis.LogsStream != "" {
		streams = append(streams, hs.config.Data.Kinesis.LogsStream)
	}

	for _, streamName := range streams {
		input := &kinesis.DescribeStreamInput{
			StreamName: &streamName,
		}

		output, err := kinesisClient.DescribeStream(ctx, input)
		if err != nil {
			return fmt.Errorf("kinesis stream '%s' health check failed: %w", streamName, err)
		}

		if output.StreamDescription == nil || output.StreamDescription.StreamStatus != kinesistypes.StreamStatusActive {
			return fmt.Errorf("kinesis stream '%s' is not active", streamName)
		}

		streamCount++
		// Only check the first stream for basic connectivity
		if streamCount >= MaxStreamCheckCount {
			break
		}
	}

	return nil
}

// collectHealthResults gathers results from async health checks
func (hs *HealthService) collectHealthResults(ctx context.Context, resultsChan <-chan HealthCheckResult, activeChecks int, response *HealthResponse) {
	for i := MinConcurrentChecks; i < activeChecks; i++ {
		select {
		case result := <-resultsChan:
			response.Dependencies[result.DependencyName] = result.Status
			if status, exists := result.Status[constants.HealthFieldStatus].(string); exists && status == constants.HealthStatusUnhealthy {
				response.OverallStatus = constants.HealthStatusUnhealthy
			}
		case <-ctx.Done():
			// Handle timeout for remaining checks
			logger.Warn("Health check timeout occurred")
			response.OverallStatus = constants.HealthStatusUnhealthy

			// Mark remaining dependencies as unhealthy due to timeout
			if _, exists := response.Dependencies[constants.HealthDependencyDatabase]; !exists && hs.config.Data.ClickHouse.Host != "" {
				response.Dependencies[constants.HealthDependencyDatabase] = gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckTimeout,
				}
			}
			if _, exists := response.Dependencies[constants.HealthDependencyKinesis]; !exists && (hs.config.Data.Kinesis.TracesStream != "" || hs.config.Data.Kinesis.MetricsStream != "" || hs.config.Data.Kinesis.LogsStream != "") {
				response.Dependencies[constants.HealthDependencyKinesis] = gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckTimeout,
				}
			}
			if _, exists := response.Dependencies[constants.HealthDependencyConnectionPool]; !exists && hs.clickHouseClient != nil {
				response.Dependencies[constants.HealthDependencyConnectionPool] = gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckTimeout,
				}
			}
			return
		}
	}
}

// Helper functions
func getRuntimeVersion() string {
	version := runtime.Version()
	// Sanitize version string to remove any potential sensitive info
	sanitized := constants.VersionSanitizeRegex.ReplaceAllString(version, "")
	if sanitized == "" {
		return constants.ErrorUnknownVersion
	}
	return sanitized
}

func getMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / float64(BytesPerMegabyte)
}


// checkConnectionPoolHealth performs connection pool health check
func (hs *HealthService) checkConnectionPoolHealth(ctx context.Context, resultsChan chan<- HealthCheckResult) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Connection pool health check panic recovered", zap.Any("panic", r))
			resultsChan <- HealthCheckResult{
				DependencyName: constants.HealthDependencyConnectionPool,
				Status: gin.H{
					constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
					constants.HealthFieldError:  ErrHealthCheckPanic,
				},
			}
		}
	}()

	if err := hs.performConnectionPoolCheck(ctx); err != nil {
		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyConnectionPool,
			Status: gin.H{
				constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
				constants.HealthFieldError:  err.Error(),
			},
		}
	} else {
		// Get detailed pool stats for the health response
		poolStats, statsErr := hs.clickHouseClient.GetConnectionPoolStats(ctx)
		status := gin.H{
			constants.HealthFieldStatus: constants.HealthStatusHealthy,
		}

		if statsErr == nil {
			status["utilization_percent"] = poolStats.UtilizationPercent
			status["open_connections"] = poolStats.OpenConnections
			status["max_connections"] = poolStats.MaxOpenConnections
			status["idle_connections"] = poolStats.IdleConnections
			status["pool_health_status"] = poolStats.HealthStatus
		}

		resultsChan <- HealthCheckResult{
			DependencyName: constants.HealthDependencyConnectionPool,
			Status:         status,
		}
	}
}

// performConnectionPoolCheck executes the actual connection pool health test
func (hs *HealthService) performConnectionPoolCheck(ctx context.Context) error {
	if hs.clickHouseClient == nil {
		return fmt.Errorf("ClickHouse client not configured for connection pool monitoring")
	}

	// Perform the connection pool health check
	if err := hs.clickHouseClient.CheckConnectionPoolHealth(ctx); err != nil {
		return fmt.Errorf("connection pool health check failed: %w", err)
	}

	return nil
}