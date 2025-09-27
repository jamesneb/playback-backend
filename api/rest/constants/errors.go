package constants

// Error Message constants for REST API
const (
	ErrorDependencyKeyTimeout   = "dependency key computation timeout"
	ErrorTraceHandlerCreation   = "failed to create trace handler"
	ErrorMetricsHandlerCreation = "failed to create metrics handler"
	ErrorLogsHandlerCreation    = "failed to create logs handler"
	ErrorReplayHandlerCreation  = "failed to create replay handler"
	ErrorDependenciesNil        = "dependencies cannot be nil"
	ErrorConfigNil              = "config cannot be nil"
	ErrorEndpointsNil           = "endpoints cannot be nil"
	ErrorGinServerCreation      = "gin server creation failed, possible memory issue"
	ErrorConfigMiddlewareNil    = "config cannot be nil for middleware setup"
	ErrorNilPanicMessage        = "nil panic"
	ErrorUnknownVersion         = "unknown"
)
