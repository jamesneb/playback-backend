package app

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/jamesneb/playback-backend/internal/config/base"
	"github.com/jamesneb/playback-backend/internal/config/decodeutil"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
)

type Config struct {
	Name        string           `mapstructure:"app_name"`
	Version     *semver.Version  `mapstructure:"version"`
	Environment base.Environment `mapstructure:"environment"`
	LogLevel    base.LogLevel    `mapstructure:"log_level"`
	LogFormat   base.LogFormat   `mapstructure:"log_format"`
}

func Defaults() Config {
	return Config{
		Name:        DEFAULT_APP_NAME,
		Version:     DEFAULT_APP_VERSION,
		Environment: DEFAULT_APP_ENVIRONMENT,
		LogLevel:    DEFAULT_APP_LOG_LEVEL,
		LogFormat:   DEFAULT_APP_LOG_FORMAT,
	}
}

func (c Config) Validate() error {
	v := base.NewValidator("APP")
	return v.Err()
}

func FromResolver(r resolver.PropertyResolver) (Config, error) {
	cfg := Defaults()
	if err := decodeutil.DecodePrefixInto(r, APP_PREFIX, &cfg); err != nil {
		return Config{}, fmt.Errorf("app decode: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
