package clickhouse

import (
	"time"
)

const (
	CLICKHOUSE_PREFIX = "CLICKHOUSE_"
)

// Connection constants
const (
	MIN_MAX_CONNECTIONS     = 1
	MAX_MAX_CONNECTIONS     = 1000
	DEFAULT_MAX_CONNECTIONS = 10

	MIN_MAX_IDLE_CONNECTIONS     = 1
	MAX_MAX_IDLE_CONNECTIONS     = 1000
	DEFAULT_MAX_IDLE_CONNECTIONS = 5

	MIN_CONNECTION_TIMEOUT     = 1 * time.Second
	MAX_CONNECTION_TIMEOUT     = 5 * time.Minute
	DEFAULT_CONNECTION_TIMEOUT = 30 * time.Second

	MIN_CONNECTION_MAX_LIFETIME     = 1 * time.Minute
	MAX_CONNECTION_MAX_LIFETIME     = 24 * time.Hour
	DEFAULT_CONNECTION_MAX_LIFETIME = 30 * time.Minute
)

// Default values
const (
	DEFAULT_HOST                      = "localhost:9000"
	DEFAULT_HTTP_HOST                 = "localhost:8123"
	DEFAULT_DATABASE                  = "telemetry"
	DEFAULT_USERNAME                  = "default"
	DEFAULT_ENABLE_COMPRESSION        = true
	DEFAULT_ENABLE_CONNECTION_POOLING = true
	DEFAULT_ENABLE_QUERY_LOGGING      = false
)
