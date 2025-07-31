package streaming

import (
	"context"
	"encoding/json"
	"time"
)

// Handler interface for telemetry data processing
type Handler interface {
	HandleTelemetryEvent(ctx context.Context, event *TelemetryEvent) error
}

// Updated TelemetryEvent structure to match what gRPC services expect
type TelemetryEvent struct {
	Type        string                `json:"type"`         // "traces", "metrics", "logs"
	ServiceName string                `json:"service_name"` // for partitioning
	TraceID     string                `json:"trace_id,omitempty"`
	Data        json.RawMessage       `json:"data"`         // Raw JSON data preserved
	Metadata    TelemetryMetadata     `json:"metadata"`
}

type TelemetryMetadata struct {
	IngestedAt time.Time `json:"ingested_at"`
	SourceIP   string    `json:"source_ip"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Version    string    `json:"version,omitempty"`
}

// KinesisHandler implements Handler interface for Kinesis streaming
type KinesisHandler struct {
	client *KinesisClient
}

func NewKinesisHandler(client *KinesisClient) *KinesisHandler {
	return &KinesisHandler{
		client: client,
	}
}

func (h *KinesisHandler) HandleTelemetryEvent(ctx context.Context, event *TelemetryEvent) error {
	switch event.Type {
	case "traces":
		return h.client.PublishTrace(ctx, event.Data, event.ServiceName, event.TraceID, 
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	case "metrics":
		return h.client.PublishMetrics(ctx, event.Data, event.ServiceName, 
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	case "logs":
		return h.client.PublishLogs(ctx, event.Data, event.ServiceName, event.TraceID,
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	default:
		return nil // Ignore unknown types
	}
}