package s3

import (
	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	S3_PREFIX = "S3_"
)

// Default values
const (
	DEFAULT_REGION           = base.AWS_US_EAST_1
	DEFAULT_FORCE_PATH_STYLE = false
	DEFAULT_ENABLE_SSE       = true
)
