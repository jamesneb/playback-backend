// internal/config/decodeutil/decode.go
//
// Package decodeutil provides utility functions for working efficiently with property resolvers.
//
// We map PREFIX_FOO_BAR → struct fields via mapstructure tags (defaulting to snake of the field),
// with these hooks (duration, bool, int, CSV, Port, Byte).
// Unknown keys are ignored; missing keys are defaults; we don’t fetch from external sources

package decodeutil
