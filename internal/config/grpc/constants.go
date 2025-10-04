package grpc

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Default config values
const (
	DEFAULT_GRPC_SERVER_PORT = base.Port(4317)
	MIN_PORT                 = base.Port(1)
	MAX_PORT                 = base.Port(65535)

	DEFAULT_MAX_RECEIVE_SIZE       = base.Byte(base.MEGA * 16)
	DEFAULT_MAX_SEND_SIZE          = base.Byte(base.MEGA * 16)
	DEFAULT_CONNECTION_TIMEOUT     = 30 * time.Second
	MIN_CONNECTION_TIMEOUT         = 100 * time.Millisecond
	MAX_CONNECTION_TIMEOUT         = 30 * time.Second
	DEFAULT_REQUESTS_PER_SECOND    = 100
	RATE_LIMIT_DISABLED            = 0
	MIN_RPS                        = 1
	MAX_RPS                        = 200_000
	DEFAULT_REQUEST_BURST_CAPACITY = 200
	MIN_RBC                        = 1
	MAX_RBC                        = 1_000_000
	MAX_BURST_MULTIPLIER           = 2
)

// Default TLS values
const (
	DEFAULT_TLS_ENABLED     = false
	DEFAULT_TLS_MIN_VERSION = base.TLS_1_2
	DEFAULT_TLS_MAX_VERSION = base.TLS_1_3
)

// Default token authentication values
const (
	DEFAULT_ENABLE_TOKEN_AUTH = false
)

// Config key names
const (
	GRPC_PREFIX string = "GRPC_"
	KEY_PORT    string = GRPC_PREFIX + "SERVER_PORT"
	KEY_MAX_REC string = GRPC_PREFIX + "MAX_RECEIVE_SIZE"
)
