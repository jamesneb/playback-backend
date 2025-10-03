package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesneb/playback-backend/internal/handlers/dto"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Query constants to eliminate magic strings
	DefaultLogsLimit    = 100
	MaxLogsLimit        = 10000
	DefaultLogsOffset   = 0
	DefaultQueryTimeout = 30 * time.Second

	// ClickHouse table name for logs
	LogsTableName = "logs"
)

// LogsQueryService handles log querying logic
type LogsQueryService interface {
	QueryLogs(ctx context.Context, params LogsQueryParams) (*dto.LogsQueryResponse, error)
}

// LogsQueryParams represents parameters for log queries
type LogsQueryParams struct {
	Service string
	Level   string
	From    string
	To      string
	Query   string
	Limit   int
	Offset  int
}

// ClickHouseLogsQueryService provides ClickHouse-based log querying
type ClickHouseLogsQueryService struct {
	clickhouse *storage.ClickHouseClient
	logger     *logger.Logger
}

// NewClickHouseLogsQueryService creates a new ClickHouse-based logs query service
func NewClickHouseLogsQueryService(clickhouse *storage.ClickHouseClient) LogsQueryService {
	return &ClickHouseLogsQueryService{
		clickhouse: clickhouse,
		logger:     logger.GetGlobalLogger(),
	}
}

// QueryLogs performs the log query using ClickHouse
func (s *ClickHouseLogsQueryService) QueryLogs(ctx context.Context, params LogsQueryParams) (*dto.LogsQueryResponse, error) {
	// Validate and set defaults
	if params.Limit <= 0 || params.Limit > MaxLogsLimit {
		params.Limit = DefaultLogsLimit
	}
	if params.Offset < 0 {
		params.Offset = DefaultLogsOffset
	}

	queryCtx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	// Build dynamic query based on parameters
	whereConditions := []string{"1=1"}
	args := []interface{}{}

	if params.Service != "" {
		whereConditions = append(whereConditions, "service_name = ?")
		args = append(args, params.Service)
	}

	if params.Level != "" {
		whereConditions = append(whereConditions, "level = ?")
		args = append(args, strings.ToUpper(params.Level))
	}

	// Parse time range parameters
	if params.From != "" {
		if fromTime, err := time.Parse(time.RFC3339, params.From); err == nil {
			whereConditions = append(whereConditions, "timestamp >= ?")
			args = append(args, fromTime)
		} else {
			logger.Warn("Invalid from time format", zap.String("from", params.From))
		}
	}

	if params.To != "" {
		if toTime, err := time.Parse(time.RFC3339, params.To); err == nil {
			whereConditions = append(whereConditions, "timestamp <= ?")
			args = append(args, toTime)
		} else {
			logger.Warn("Invalid to time format", zap.String("to", params.To))
		}
	}

	// Add text search if query is provided
	if params.Query != "" {
		whereConditions = append(whereConditions, "positionCaseInsensitive(message, ?) > 0")
		args = append(args, params.Query)
	}

	// Build the main query
	query := fmt.Sprintf(`
		SELECT
			timestamp,
			level,
			message,
			service_name,
			trace_id,
			span_id,
			attributes
		FROM %s
		WHERE %s
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, LogsTableName, strings.Join(whereConditions, " AND "))

	// Add limit and offset to args
	args = append(args, params.Limit, params.Offset)

	rows, err := s.clickhouse.QueryWithArgs(queryCtx, query, args...)
	if err != nil {
		logger.Error("Failed to query logs",
			zap.Error(err),
			zap.String("service", params.Service),
			zap.String("level", params.Level),
			zap.String("query", params.Query))
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	var logs []dto.LogEntry
	for rows.Next() {
		var log dto.LogEntry
		var traceID, spanID, attributesJSON *string

		if err := rows.Scan(
			&log.Timestamp,
			&log.Level,
			&log.Message,
			&log.Service,
			&traceID,
			&spanID,
			&attributesJSON,
		); err != nil {
			logger.Error("Failed to scan log row", zap.Error(err))
			continue
		}

		// Set optional fields
		if traceID != nil {
			log.TraceID = *traceID
		}
		if spanID != nil {
			log.SpanID = *spanID
		}

		// Parse attributes JSON if present
		if attributesJSON != nil && *attributesJSON != "" {
			// TODO: Parse JSON attributes into map[string]interface{}
			log.Attributes = make(map[string]interface{})
		}

		logs = append(logs, log)
	}

	return &dto.LogsQueryResponse{
		Service:   params.Service,
		Level:     params.Level,
		TimeRange: dto.TimeRange{From: params.From, To: params.To},
		Query:     params.Query,
		Logs:      logs,
	}, nil
}

// DefaultLogsQueryService provides a fallback that returns errors for production safety
type DefaultLogsQueryService struct{}

// NewDefaultLogsQueryService creates a new default logs query service
func NewDefaultLogsQueryService() LogsQueryService {
	return &DefaultLogsQueryService{}
}

// QueryLogs returns an error indicating ClickHouse is required for production queries
func (s *DefaultLogsQueryService) QueryLogs(ctx context.Context, params LogsQueryParams) (*dto.LogsQueryResponse, error) {
	return nil, fmt.Errorf("ClickHouse connection required for log queries in production")
}
