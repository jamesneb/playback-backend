package rest

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// Dependencies holds all the dependencies needed by the REST API
type Dependencies struct {
	Config               *config.Config
	KinesisClient        *streaming.KinesisClient
	ClickHouseClient     *storage.ClickHouseClient
	S3Client             *s3.Client
	Endpoints            *api.EndpointCollection
	ResilienceComponents *interfaces.ResilienceComponents
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
	traceHandler := handlers.NewTraceHandler(deps.KinesisClient, deps.ResilienceComponents)
	if traceHandler == nil {
		return nil, errors.New(constants.ErrorTraceHandlerCreation)
	}

	metricsHandler := handlers.NewMetricsHandler(deps.KinesisClient)
	if metricsHandler == nil {
		return nil, errors.New(constants.ErrorMetricsHandlerCreation)
	}

	logsHandler := handlers.NewLogsHandler(deps.KinesisClient)
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
