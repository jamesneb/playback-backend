package redis

import (
	"time"
)

const (
	REDIS_PREFIX = "REDIS_"
)

// Time period constants
const (
	ONE_DAY = 24 * time.Hour
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
	MAX_CONNECTION_TIMEOUT     = 1 * time.Minute
	DEFAULT_CONNECTION_TIMEOUT = 5 * time.Second

	MIN_CONNECTION_MAX_LIFETIME     = 1 * time.Minute
	MAX_CONNECTION_MAX_LIFETIME     = ONE_DAY
	DEFAULT_CONNECTION_MAX_LIFETIME = 30 * time.Minute
)

// TTL constants
const (
	MIN_DEFAULT_TTL     = 1 * time.Second
	MAX_DEFAULT_TTL     = ONE_DAY
	DEFAULT_DEFAULT_TTL = 5 * time.Minute
)

// Database index constants (Redis supports 0-15 databases by default)
const (
	MIN_DATABASE_INDEX     = 0
	MAX_DATABASE_INDEX     = 15
	DEFAULT_DATABASE_INDEX = 0
)

// Default values
const (
	DEFAULT_HOST                      = "localhost:6379"
	DEFAULT_ENABLE_CONNECTION_POOLING = true
)
