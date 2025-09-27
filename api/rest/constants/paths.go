package constants

// Type aliases for paths
type PathSegment string

// Path constants
const (
	RootPath            PathSegment = "/"
	HealthEndpointPath  PathSegment = "/health"
	MetricsEndpointPath PathSegment = "/metrics"
	DebugPathPrefix     PathSegment = "/debug"
	SwaggerPathSegment  PathSegment = "swagger"
)
