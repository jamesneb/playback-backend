package rest

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/metrics"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
)

// Dependencies holds all the dependencies needed by the REST API
type Dependencies struct {
	Config               *config.ConsolidatedConfig
	KinesisClient        *streaming.KinesisClient
	ClickHouseClient     *storage.ClickHouseClient
	S3Client             *s3.Client
	Endpoints            *api.EndpointCollection
	ResilienceComponents *interfaces.ResilienceComponents
	MetricsRegistry      *metrics.Registry
}

// APIHandlers holds all HTTP handlers for the REST API
type APIHandlers struct {
	Trace   *handlers.TraceHandler
	Metrics *handlers.MetricsHandler
	Logs    *handlers.LogsHandler
	Replay  *handlers.ReplayHandler
}

// NewAPIHandlers creates API handlers directly without caching complexity
func NewAPIHandlers(deps *Dependencies) (*APIHandlers, error) {
	if deps == nil {
		return nil, errors.New(constants.ErrorDependenciesNil)
	}

	return createHandlers(deps)
}

// createHandlers creates a new set of API handlers
func createHandlers(deps *Dependencies) (*APIHandlers, error) {
	// Use stub publisher for local development when Kinesis is not available
	var eventPublisher telemetry.EventPublisher
	if deps.KinesisClient == nil {
		logger.Info("Using stub event publisher for local development")
		eventPublisher = streaming.NewStubEventPublisher()
	} else {
		eventPublisher = deps.KinesisClient
	}

	// Create handlers with ClickHouse integration for production queries
	var traceHandler *handlers.TraceHandler
	var metricsHandler *handlers.MetricsHandler
	var logsHandler *handlers.LogsHandler

	if deps.ClickHouseClient != nil {
		// Production: Use real ClickHouse query services
		traceHandler = handlers.NewTraceHandlerWithClickHouse(eventPublisher, deps.ResilienceComponents, deps.ClickHouseClient)
		metricsHandler = handlers.NewMetricsHandlerWithClickHouse(eventPublisher, deps.ClickHouseClient, deps.ResilienceComponents)
		logsHandler = handlers.NewLogsHandlerWithClickHouse(eventPublisher, deps.ClickHouseClient, deps.ResilienceComponents)
	} else {
		// Fallback: Use handlers without query capabilities (ingestion only)
		traceHandler = handlers.NewTraceHandler(eventPublisher, deps.ResilienceComponents)
		metricsHandler = handlers.NewMetricsHandler(eventPublisher, deps.ResilienceComponents)
		logsHandler = handlers.NewLogsHandler(eventPublisher, deps.ResilienceComponents)
	}

	if traceHandler == nil {
		return nil, errors.New(constants.ErrorTraceHandlerCreation)
	}
	if metricsHandler == nil {
		return nil, errors.New(constants.ErrorMetricsHandlerCreation)
	}
	if logsHandler == nil {
		return nil, errors.New(constants.ErrorLogsHandlerCreation)
	}

	var replayHandler *handlers.ReplayHandler
	if deps.S3Client != nil {
		replayHandler = handlers.NewReplayHandler(deps.S3Client, constants.ReplayS3BucketName)
		if replayHandler == nil {
			return nil, errors.New(constants.ErrorReplayHandlerCreation)
		}
	}

	return &APIHandlers{
		Trace:   traceHandler,
		Metrics: metricsHandler,
		Logs:    logsHandler,
		Replay:  replayHandler,
	}, nil
}
