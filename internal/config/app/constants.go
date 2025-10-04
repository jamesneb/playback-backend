package app

import (
	"github.com/Masterminds/semver/v3"
	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Default config value constants
const (
	APP_PREFIX              string           = "APP_"
	DEFAULT_APP_NAME        string           = "playback"
	DEFAULT_APP_LOG_LEVEL   base.LogLevel    = base.LOG_INFO
	DEFAULT_APP_LOG_FORMAT  base.LogFormat   = base.LOG_JSON
	DEFAULT_APP_ENVIRONMENT base.Environment = base.DEV_ENV
)

// vars, init-time parse
var DEFAULT_APP_VERSION = semver.MustParse("0.0.0")
