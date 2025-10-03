// Package base is used throughout the config module and contains types/functions universal to
// the cofiguration system
//
// functions.go defines universal config types
package base

import (
	"errors"
	"fmt"
	"hash/maphash"
	"maps"
	"slices"
	"strings"
)

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
