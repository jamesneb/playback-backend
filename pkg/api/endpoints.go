package api

import (
	"fmt"
)

// APIEndpoints contains all API endpoint definitions
// This prevents hardcoding URLs throughout the codebase
type APIEndpoints struct {
	BaseURL    string
	APIVersion string
}

// NewAPIEndpoints creates a new endpoint collection
func NewAPIEndpoints(baseURL, version string) *APIEndpoints {
	return &APIEndpoints{
		BaseURL:    baseURL,
		APIVersion: version,
	}
}

// Base paths
func (e *APIEndpoints) BasePath() string {
	return fmt.Sprintf("/api/%s", e.APIVersion)
}

// Health and system endpoints
func (e *APIEndpoints) Health() string {
	return fmt.Sprintf("%s/health", e.BasePath())
}

func (e *APIEndpoints) Ready() string {
	return fmt.Sprintf("%s/ready", e.BasePath())
}

func (e *APIEndpoints) Metrics() string {
	return "/metrics"
}

// OpenTelemetry HTTP endpoints (legacy)
func (e *APIEndpoints) Traces() string {
	return fmt.Sprintf("%s/traces", e.BasePath())
}

func (e *APIEndpoints) TracesRelative() string {
	return "/traces"
}

func (e *APIEndpoints) TraceByID(id string) string {
	if id == "" {
		return fmt.Sprintf("%s/traces/:id", e.BasePath())
	}
	return fmt.Sprintf("%s/traces/%s", e.BasePath(), id)
}

func (e *APIEndpoints) TraceByIDRelative() string {
	return "/traces/:id"
}

func (e *APIEndpoints) TracesCreate() string {
	return fmt.Sprintf("%s/traces", e.BasePath())
}

// Metrics endpoints
func (e *APIEndpoints) MetricsEndpoint() string {
	return fmt.Sprintf("%s/metrics", e.BasePath())
}

func (e *APIEndpoints) MetricsRelative() string {
	return "/metrics"
}

func (e *APIEndpoints) MetricsCreate() string {
	return fmt.Sprintf("%s/metrics", e.BasePath())
}

// Logs endpoints
func (e *APIEndpoints) Logs() string {
	return fmt.Sprintf("%s/logs", e.BasePath())
}

func (e *APIEndpoints) LogsRelative() string {
	return "/logs"
}

func (e *APIEndpoints) LogsCreate() string {
	return fmt.Sprintf("%s/logs", e.BasePath())
}

// Replay endpoints
func (e *APIEndpoints) ReplaysList() string {
	return fmt.Sprintf("%s/replays/list", e.BasePath())
}

func (e *APIEndpoints) ReplaysListRelative() string {
	return "/replays/list"
}

func (e *APIEndpoints) ReplaysDownload() string {
	return fmt.Sprintf("%s/replays/download", e.BasePath())
}

func (e *APIEndpoints) ReplaysDownloadRelative() string {
	return "/replays/download"
}

func (e *APIEndpoints) ReplayByID(id string) string {
	if id == "" {
		return fmt.Sprintf("%s/replays/:id", e.BasePath())
	}
	return fmt.Sprintf("%s/replays/%s", e.BasePath(), id)
}

// Swagger documentation
func (e *APIEndpoints) SwaggerUI() string {
	return "/swagger/*any"
}

func (e *APIEndpoints) SwaggerJSON() string {
	return "/swagger/swagger.json"
}

// Monitoring and debug endpoints
func (e *APIEndpoints) DebugPprof() string {
	return "/debug/pprof/*any"
}

func (e *APIEndpoints) PrometheusMetrics() string {
	return "/metrics"
}

// Full URL construction
func (e *APIEndpoints) FullURL(endpoint string) string {
	if e.BaseURL == "" {
		return endpoint
	}
	return fmt.Sprintf("%s%s", e.BaseURL, endpoint)
}

// Common endpoint collections for different use cases
type EndpointCollection struct {
	*APIEndpoints
}

// NewEndpointCollection creates a new collection with default settings
func NewEndpointCollection(baseURL string) *EndpointCollection {
	return &EndpointCollection{
		APIEndpoints: NewAPIEndpoints(baseURL, "v1"),
	}
}

// GetAllEndpoints returns a map of all endpoints for documentation/debugging
func (e *APIEndpoints) GetAllEndpoints() map[string]string {
	return map[string]string{
		// System
		"health":           e.Health(),
		"ready":            e.Ready(),
		"metrics":          e.Metrics(),
		
		// OpenTelemetry
		"traces":           e.Traces(),
		"traces_create":    e.TracesCreate(),
		"traces_by_id":     e.TraceByID(""),
		"metrics_endpoint": e.MetricsEndpoint(),
		"metrics_create":   e.MetricsCreate(),
		"logs":             e.Logs(),
		"logs_create":      e.LogsCreate(),
		
		// Replays
		"replays_list":     e.ReplaysList(),
		"replays_download": e.ReplaysDownload(),
		"replays_by_id":    e.ReplayByID(""),
		
		// Documentation
		"swagger_ui":       e.SwaggerUI(),
		"swagger_json":     e.SwaggerJSON(),
		
		// Debug
		"debug_pprof":      e.DebugPprof(),
		"prometheus":       e.PrometheusMetrics(),
	}
}

// Default endpoints instance for easy access
var DefaultEndpoints = NewEndpointCollection("")