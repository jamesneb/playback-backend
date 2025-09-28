package dto

import "time"

// LogsResponse represents the response after log ingestion
type LogsResponse struct {
	Received  int       `json:"received" example:"10"`
	Timestamp time.Time `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Status    string    `json:"status" example:"accepted"`
}

// LogsQueryResponse represents the response for log queries
type LogsQueryResponse struct {
	Service   string     `json:"service" example:"order-service"`
	Level     string     `json:"level" example:"INFO"`
	TimeRange TimeRange  `json:"time_range"`
	Query     string     `json:"query" example:"order failed"`
	Logs      []LogEntry `json:"logs"`
}

// LogEntry represents a single log entry in query results
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Level      string                 `json:"level" example:"INFO"`
	Message    string                 `json:"message" example:"Order creation started"`
	Service    string                 `json:"service" example:"order-service"`
	TraceID    string                 `json:"trace_id,omitempty" example:"abc123def456"`
	SpanID     string                 `json:"span_id,omitempty" example:"789xyz"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TimeRange represents a time range for queries
type TimeRange struct {
	From string `json:"from" example:"2023-01-01T00:00:00Z"`
	To   string `json:"to" example:"2023-01-01T23:59:59Z"`
}
