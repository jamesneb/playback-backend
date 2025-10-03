package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Query constants to eliminate magic strings
	DefaultMetricsLimit  = 100
	MaxMetricsLimit      = 10000
	DefaultMetricsOffset = 0

	// ClickHouse table name for metrics
	MetricsTableName = "metrics"
)

// MetricsQueryService handles metrics querying logic
type MetricsQueryService interface {
	QueryMetrics(ctx context.Context, params MetricsQueryParams) (*MetricsQueryResponse, error)
}

// MetricsQueryParams represents parameters for metrics queries
type MetricsQueryParams struct {
	ServiceName string
	MetricName  string
	From        time.Time
	To          time.Time
	Aggregation string // sum, avg, count, min, max
	GroupBy     string // service, metric_name, time_bucket
	Limit       int
	Offset      int
}

// MetricsQueryResponse represents metrics query response
type MetricsQueryResponse struct {
	Service   string        `json:"service"`
	TimeRange TimeRange     `json:"time_range"`
	Metrics   []MetricData  `json:"metrics"`
	QueryTime time.Duration `json:"query_time_ms"`
}

// MetricData represents a single metric data point
type MetricData struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Value      float64                `json:"value"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TimeRange represents a time range for queries
type TimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ClickHouseMetricsQueryService provides ClickHouse-based metrics querying
type ClickHouseMetricsQueryService struct {
	clickhouse *storage.ClickHouseClient
	logger     *logger.Logger
}

// NewClickHouseMetricsQueryService creates a new ClickHouse-based metrics query service
func NewClickHouseMetricsQueryService(clickhouse *storage.ClickHouseClient) MetricsQueryService {
	return &ClickHouseMetricsQueryService{
		clickhouse: clickhouse,
		logger:     logger.GetGlobalLogger(),
	}
}

// QueryMetrics performs the metrics query using ClickHouse
func (s *ClickHouseMetricsQueryService) QueryMetrics(ctx context.Context, params MetricsQueryParams) (*MetricsQueryResponse, error) {
	queryStart := time.Now()

	// Validate and set defaults
	if params.Limit <= 0 || params.Limit > MaxMetricsLimit {
		params.Limit = DefaultMetricsLimit
	}
	if params.Offset < 0 {
		params.Offset = DefaultMetricsOffset
	}

	queryCtx, cancel := context.WithTimeout(ctx, DefaultQueryTimeout)
	defer cancel()

	// Build dynamic query based on parameters
	whereConditions := []string{"1=1"}
	args := []interface{}{}

	if params.ServiceName != "" {
		whereConditions = append(whereConditions, "service_name = ?")
		args = append(args, params.ServiceName)
	}

	if params.MetricName != "" {
		whereConditions = append(whereConditions, "metric_name = ?")
		args = append(args, params.MetricName)
	}

	if !params.From.IsZero() {
		whereConditions = append(whereConditions, "timestamp >= ?")
		args = append(args, params.From)
	}

	if !params.To.IsZero() {
		whereConditions = append(whereConditions, "timestamp <= ?")
		args = append(args, params.To)
	}

	// Build aggregation and grouping
	selectFields := []string{
		"metric_name",
		"service_name",
		"timestamp",
		"metric_type",
		"value",
		"labels",
		"attributes",
	}

	// Apply aggregation if specified
	if params.Aggregation != "" {
		switch strings.ToLower(params.Aggregation) {
		case "sum":
			selectFields[4] = "sum(value) as value"
		case "avg":
			selectFields[4] = "avg(value) as value"
		case "count":
			selectFields[4] = "count(*) as value"
		case "min":
			selectFields[4] = "min(value) as value"
		case "max":
			selectFields[4] = "max(value) as value"
		}
	}

	// Build GROUP BY clause if aggregation is used
	groupByClause := ""
	if params.Aggregation != "" {
		groupFields := []string{"metric_name", "service_name", "metric_type"}
		if params.GroupBy != "" {
			switch strings.ToLower(params.GroupBy) {
			case "service":
				groupFields = []string{"service_name"}
			case "metric_name":
				groupFields = []string{"metric_name"}
			case "time_bucket":
				// Group by 5-minute intervals
				selectFields[2] = "toStartOfInterval(timestamp, INTERVAL 5 MINUTE) as timestamp"
				groupFields = append(groupFields, "toStartOfInterval(timestamp, INTERVAL 5 MINUTE)")
			}
		}
		groupByClause = " GROUP BY " + strings.Join(groupFields, ", ")
	}

	// Build the main query
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s
		WHERE %s
		%s
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, strings.Join(selectFields, ",\n\t\t\t"), MetricsTableName, strings.Join(whereConditions, " AND "), groupByClause)

	// Add limit and offset to args
	args = append(args, params.Limit, params.Offset)

	rows, err := s.clickhouse.QueryWithArgs(queryCtx, query, args...)
	if err != nil {
		logger.Error("Failed to query metrics",
			zap.Error(err),
			zap.String("service", params.ServiceName),
			zap.String("metric_name", params.MetricName))
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	var metrics []MetricData
	for rows.Next() {
		var metric MetricData
		var serviceName string
		var labelsJSON, attributesJSON *string

		if err := rows.Scan(
			&metric.Name,
			&serviceName,
			&metric.Timestamp,
			&metric.Type,
			&metric.Value,
			&labelsJSON,
			&attributesJSON,
		); err != nil {
			logger.Error("Failed to scan metric row", zap.Error(err))
			continue
		}

		// Parse labels JSON if present
		if labelsJSON != nil && *labelsJSON != "" {
			// TODO: Parse JSON labels into map[string]string
			metric.Labels = make(map[string]string)
		}

		// Parse attributes JSON if present
		if attributesJSON != nil && *attributesJSON != "" {
			// TODO: Parse JSON attributes into map[string]interface{}
			metric.Attributes = make(map[string]interface{})
		}

		metrics = append(metrics, metric)
	}

	queryTime := time.Since(queryStart)

	// Format time range strings
	var fromStr, toStr string
	if !params.From.IsZero() {
		fromStr = params.From.Format(time.RFC3339)
	}
	if !params.To.IsZero() {
		toStr = params.To.Format(time.RFC3339)
	}

	return &MetricsQueryResponse{
		Service:   params.ServiceName,
		TimeRange: TimeRange{From: fromStr, To: toStr},
		Metrics:   metrics,
		QueryTime: queryTime,
	}, nil
}

// DefaultMetricsQueryService provides a fallback that returns errors for production safety
type DefaultMetricsQueryService struct{}

// NewDefaultMetricsQueryService creates a new default metrics query service
func NewDefaultMetricsQueryService() MetricsQueryService {
	return &DefaultMetricsQueryService{}
}

// QueryMetrics returns an error indicating ClickHouse is required for production queries
func (s *DefaultMetricsQueryService) QueryMetrics(ctx context.Context, params MetricsQueryParams) (*MetricsQueryResponse, error) {
	return nil, fmt.Errorf("ClickHouse connection required for metrics queries in production")
}
