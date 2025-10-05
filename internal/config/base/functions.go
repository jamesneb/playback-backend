// functions.go defines universal config types
package base

import (
	"errors"
	"fmt"
	"hash/maphash"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// returns stringified versions of log levels
func (l LogLevel) String() string {
	switch l {
	case LOG_DEBUG:
		return "debug"
	case LOG_INFO:
		return "info"
	case LOG_WARN:
		return "warn"
	case LOG_ERR:
		return "error"
	case LOG_FATAL:
		return "fatal"
	default:
		return "unknown"
	}
}

// returns a stringified version of the logging format
func (f LogFormat) String() string {
	switch f {
	case LOG_JSON:
		return "json"
	case LOG_CONSOLE:
		return "console"
	default:
		return "unknown"
	}
}

// returns a stringified version of environment name
func (e Environment) String() string {
	switch e {
	case LOCAL_ENV:
		return "local"
	case DEV_ENV:
		return "dev"
	case STAGE_ENV:
		return "staging"
	case PROD_ENV:
		return "prod"
	case TEST_ENV:
		return "test"
	default:
		return "unknown"
	}
}

// returns a stringified version of HTTP mode
func (m HTTPMode) String() string {
	switch m {
	case HTTP_MODE_DEBUG:
		return "debug"
	case HTTP_MODE_RELEASE:
		return "release"
	case HTTP_MODE_TEST:
		return "test"
	default:
		return "unknown"
	}
}

// returns a stringified version of AWS region
func (r AWSRegion) String() string {
	switch r {
	case AWS_US_EAST_1:
		return "us-east-1"
	case AWS_US_EAST_2:
		return "us-east-2"
	case AWS_US_WEST_1:
		return "us-west-1"
	case AWS_US_WEST_2:
		return "us-west-2"
	case AWS_EU_WEST_1:
		return "eu-west-1"
	case AWS_EU_WEST_2:
		return "eu-west-2"
	case AWS_EU_CENTRAL_1:
		return "eu-central-1"
	case AWS_AP_SOUTHEAST_1:
		return "ap-southeast-1"
	case AWS_AP_SOUTHEAST_2:
		return "ap-southeast-2"
	case AWS_AP_NORTHEAST_1:
		return "ap-northeast-1"
	default:
		return "unknown"
	}
}

// returns a stringified version of TLS version
func (v TLSVersion) String() string {
	switch v {
	case TLS_1_0:
		return "1.0"
	case TLS_1_1:
		return "1.1"
	case TLS_1_2:
		return "1.2"
	case TLS_1_3:
		return "1.3"
	default:
		return "unknown"
	}
}

// returns a stringified version of data export format
func (f DataExportFormat) String() string {
	switch f {
	case DATA_EXPORT_JSON:
		return "json"
	case DATA_EXPORT_CSV:
		return "csv"
	case DATA_EXPORT_PARQUET:
		return "parquet"
	default:
		return "unknown"
	}
}

// merge merges two or more maps into a single map
func Merge(layers ...map[string]string) map[string]string {
	size := 0
	for _, m := range layers {
		size += len(m)
	}
	out := make(map[string]string, size)
	for _, m := range layers {
		// maps.Copy(dst, src) overwrites existing keys, which preserves
		// "later layer wins" as we iterate in order.
		maps.Copy(out, m)
	}
	return out
}

// FingerprintMapHash is very fast but uses a per-process random seed.
// Suitable for change detection within a process (reload loops).
// Will not be stable across processes
// Used for config change detection -- not security
func Fingerprint(m map[string]string) uint64 {
	var out uint64
	var hh maphash.Hash
	for k, v := range m {
		hh.Reset()
		hh.WriteString(k)
		hh.WriteByte(0)
		hh.WriteString(v)
		out ^= hh.Sum64()
	}
	// same avalanche as above
	out ^= out >> 33
	out *= 0xff51afd7ed558ccd
	out ^= out >> 33
	out *= 0xc4ceb9fe1a85ec53
	out ^= out >> 33
	return out
}

func Normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.ToUpper(s)
}

// buildString creates a string using a builder function with pre-allocated capacity
func buildString(capacity int, fn func(*strings.Builder)) string {
	var sb strings.Builder
	sb.Grow(capacity)
	fn(&sb)
	return sb.String()
}

// isValidPrefixChar checks if a rune is valid for a prefix (A-Z, 0-9, _)
func isValidPrefixChar(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_'
}

// SanitizePrefix removes invalid characters from prefix string and truncates to maxLen.
// Only allows A-Z, 0-9, and underscore. Uppercases result.
func SanitizePrefix(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = s[:min(len(s), maxLen)]

	// Fast path: check if already clean
	needsSanitize := false
	for _, r := range s {
		if !isValidPrefixChar(r) {
			needsSanitize = true
			break
		}
	}

	if !needsSanitize {
		return s
	}

	// Slow path: filter invalid chars and uppercase
	return buildString(len(s), func(sb *strings.Builder) {
		for _, r := range s {
			if isValidPrefixChar(r) {
				sb.WriteRune(r)
			} else if unicode.IsLower(r) {
				sb.WriteRune(unicode.ToUpper(r))
			}
		}
	})
}

// ConcatStrings efficiently concatenates two strings using strings.Builder
func ConcatStrings(a, b string) string {
	return buildString(len(a)+len(b), func(sb *strings.Builder) {
		sb.WriteString(a)
		sb.WriteString(b)
	})
}

// Create a validator for a set of config values.
// For example,validate all GRPC Server settings
func NewValidator(prefix string) *Validator { return &Validator{prefix: prefix} }

// Return a prefixed config field name
func (v *Validator) name(field string) string {
	if v.prefix == "" {
		return field
	}
	return v.prefix + "." + field
}

func (v *Validator) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return errors.Join(v.errs...)
}

// add wraps fn(), and if it returns an error, appends a labeled error.
func Add(errs *[]error, name string, fn func() error) {
	if err := fn(); err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", name, err))
	}
}

// ---- Generic helpers ----

// When runs f only if cond is true (keeps call sites tidy).
func (v *Validator) When(cond bool, f func(*Validator)) {
	if cond {
		f(v)
	}
}

// Assert appends a standardized error if ok == false.
func (v *Validator) Assert(field string, ok bool, msg string, args ...any) {
	if ok {
		return
	}
	v.errs = append(v.errs, fmt.Errorf("%s: %s",
		v.name(field), fmt.Sprintf(msg, args...)))
}

// RangeOrAllowed: if val equals any "allowed" sentinel values, accept; else enforce [min, max].
func RangeOrAllowed[T Number](v *Validator, field string, val, min, max T, unit string, allowed ...T) {
	if slices.Contains(allowed, val) {
		return
	}
	RangeFNum(v, field, val, min, max, unit)
}

// RangeNum appends a standardized out-of-bounds error when val ∉ [min, max].
func RangeNum[T Number](v *Validator, name string, val, min, max T, unit string) {
	if val < min || val > max {
		if unit != "" {
			unit = " " + unit
		}
		v.errs = append(v.errs,
			fmt.Errorf("%s out of bounds [%v, %v]%s: %v", name, min, max, unit, val))
	}
}

// RangeFNum is the prefix-aware variant: pass field names like "port", "max_send".
func RangeFNum[T Number](v *Validator, field string, val, min, max T, unit string) {
	RangeNum(v, v.name(field), val, min, max, unit)
}

// GT0Num appends an error when val ≤ 0 (for “must be positive” checks).
func GT0Num[T Number](v *Validator, name string, val T, unit string) {
	var zero T
	if val <= zero {
		if unit != "" {
			unit = " " + unit
		}
		v.errs = append(v.errs,
			fmt.Errorf("%s out of bounds (0, %s]%s: %v", name, Infinity, unit, val))
	}
}

// GT0FNum is the prefix-aware variant of GT0Num.
func GT0FNum[T Number](v *Validator, field string, val T, unit string) {
	GT0Num(v, v.name(field), val, unit)
}

// ---- Cross-validation helpers ----

// NotEqual validates that two comparable values are different
func NotEqual[T comparable](v *Validator, field string, val, other T, otherName string) {
	if val == other {
		v.errs = append(v.errs, fmt.Errorf("%s: must not equal %s (both %v)",
			v.name(field), otherName, val))
	}
}

// Equal validates that two comparable values are the same
func Equal[T comparable](v *Validator, field string, val, expected T, expectedName string) {
	if val != expected {
		v.errs = append(v.errs, fmt.Errorf("%s: must equal %s (got %v, expected %v)",
			v.name(field), expectedName, val, expected))
	}
}

// AllUnique validates that all values in a slice are unique
func AllUnique[T comparable](v *Validator, field string, vals []T) {
	seen := make(map[T]int, len(vals))
	for i, val := range vals {
		if prevIdx, exists := seen[val]; exists {
			v.errs = append(v.errs, fmt.Errorf("%s: duplicate value %v at indices %d and %d",
				v.name(field), val, prevIdx, i))
			return // Report first collision only
		}
		seen[val] = i
	}
}

// NotEmpty validates that a string field is not empty
func NotEmpty(v *Validator, field string, val string) {
	if val == "" {
		v.errs = append(v.errs, fmt.Errorf("%s: cannot be empty", v.name(field)))
	}
}

// LTE validates that a value is less than or equal to another (for coupled fields like idle <= max connections)
func LTE[T Number](v *Validator, field string, val T, max T, maxFieldName string) {
	if val > max {
		v.errs = append(v.errs, fmt.Errorf("%s: cannot exceed %s (%v > %v)",
			v.name(field), maxFieldName, val, max))
	}
}
