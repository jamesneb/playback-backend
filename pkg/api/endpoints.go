package api

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
	return "/api/" + e.APIVersion
}

// Health and system endpoints
func (e *APIEndpoints) Health() string {
	return e.BasePath() + "/health"
}

func (e *APIEndpoints) Ready() string {
	return e.BasePath() + "/ready"
}

func (e *APIEndpoints) Metrics() string {
	return "/metrics"
}

// OpenTelemetry HTTP endpoints (legacy)
func (e *APIEndpoints) Traces() string {
	return e.BasePath() + "/traces"
}

func (e *APIEndpoints) TracesRelative() string {
	return "/traces"
}

func (e *APIEndpoints) TraceByID(id string) string {
	if id == "" {
		return e.BasePath() + "/traces/:id"
	}
	return e.BasePath() + "/traces/" + id
}

func (e *APIEndpoints) TraceByIDRelative() string {
	return "/traces/:id"
}

func (e *APIEndpoints) TracesCreate() string {
	return e.BasePath() + "/traces"
}

// Metrics endpoints
func (e *APIEndpoints) MetricsEndpoint() string {
	return e.BasePath() + "/metrics"
}

func (e *APIEndpoints) MetricsRelative() string {
	return "/metrics"
}

func (e *APIEndpoints) MetricsCreate() string {
	return e.BasePath() + "/metrics"
}

// Logs endpoints  
func (e *APIEndpoints) Logs() string {
	return e.BasePath() + "/logs"
}

func (e *APIEndpoints) LogsRelative() string {
	return "/logs"
}

func (e *APIEndpoints) LogsCreate() string {
	return e.BasePath() + "/logs"
}

// Replay endpoints
func (e *APIEndpoints) ReplaysList() string {
	return e.BasePath() + "/replays/list"
}

func (e *APIEndpoints) ReplaysListRelative() string {
	return "/replays/list"
}

func (e *APIEndpoints) ReplaysDownload() string {
	return e.BasePath() + "/replays/download"
}

func (e *APIEndpoints) ReplaysDownloadRelative() string {
	return "/replays/download"
}

func (e *APIEndpoints) ReplayByID(id string) string {
	if id == "" {
		return e.BasePath() + "/replays/:id"
	}
	return e.BasePath() + "/replays/" + id
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
	return e.BaseURL + endpoint
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