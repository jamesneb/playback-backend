package rest

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		return nil, errors.New(ERROR_DEPENDENCIES_NIL)
	}

	return createHandlers(deps)
}


// createHandlers creates a new set of API handlers
func createHandlers(deps *Dependencies) (*APIHandlers, error) {
	traceHandler := handlers.NewTraceHandler(deps.KinesisClient, deps.ResilienceComponents)
	if traceHandler == nil {
		return nil, errors.New(ERROR_TRACE_HANDLER_CREATION)
	}

	metricsHandler := handlers.NewMetricsHandler(deps.KinesisClient)
	if metricsHandler == nil {
		return nil, errors.New(ERROR_METRICS_HANDLER_CREATION)
	}

	logsHandler := handlers.NewLogsHandler(deps.KinesisClient)
	if logsHandler == nil {
		return nil, errors.New(ERROR_LOGS_HANDLER_CREATION)
	}

	var replayHandler *handlers.ReplayHandler
	if deps.S3Client != nil {
		replayHandler = handlers.NewReplayHandler(deps.S3Client, REPLAY_S3_BUCKET_NAME)
		if replayHandler == nil {
			return nil, errors.New(ERROR_REPLAY_HANDLER_CREATION)
		}
	}

	return &APIHandlers{
		Trace:   traceHandler,
		Metrics: metricsHandler,
		Logs:    logsHandler,
		Replay:  replayHandler,
	}, nil
}