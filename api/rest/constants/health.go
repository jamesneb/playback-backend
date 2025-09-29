package constants

// Health check constants
const (
	HealthStatusHealthy       = "healthy"
	HealthStatusUnhealthy     = "unhealthy"
	HealthStatusNotConfigured = "not_configured"
	HealthStatusOK            = "ok"
	HealthDependencyDatabase      = "database"
	HealthDependencyKinesis       = "kinesis"
	HealthDependencyConnectionPool = "connection_pool"
	HealthFieldStatus         = "status"
	HealthFieldError          = "error"
)
