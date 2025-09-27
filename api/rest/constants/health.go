package constants

// Health check constants
const (
	HealthStatusHealthy       = "healthy"
	HealthStatusUnhealthy     = "unhealthy"
	HealthStatusNotConfigured = "not_configured"
	HealthStatusOK            = "ok"
	HealthDependencyDatabase  = "database"
	HealthDependencyKinesis   = "kinesis"
	HealthFieldStatus         = "status"
	HealthFieldError          = "error"
)
