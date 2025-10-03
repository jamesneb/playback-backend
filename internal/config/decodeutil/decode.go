// internal/config/decodeutil/decode.go
//
// Package decodeutil provides utility functions for working efficiently with property resolvers.
//
// We map PREFIX_FOO_BAR → struct fields via mapstructure tags (defaulting to snake of the field),
// with these hooks (duration, bool, int, CSV, Port, Byte).
// Unknown keys are ignored; missing keys are defaults; we don’t fetch from external sources
package decodeutil

import (
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mitchellh/mapstructure"

	"github.com/jamesneb/playback-backend/internal/config/base"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
)

var baseDecoderCfg mapstructure.DecoderConfig

func init() {
	baseDecoderCfg = mapstructure.DecoderConfig{
		TagName: "mapstructure",
		// Compose once; reused across decodes
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			StringToDurationHook(),
			StringToBoolHook(),
			StringToIntHook(),
			CSVToStringSliceHook(),
			StringToPortHook(),
			StringToByteHook(),
		),
		// Result is set per call
	}
}

func decodeSectionInto(dst any, section map[string]any) error {
	cfg := baseDecoderCfg // copy
	cfg.Result = dst
	dec, err := mapstructure.NewDecoder(&cfg)
	if err != nil {
		return err
	}
	return dec.Decode(section)
}

type enumerable interface {
	// fast-path: concrete resolvers can expose all raw pairs.
	Entries() map[string]string
	// Current prefix
	Prefix() string
}

// expectedKeysFromStructTags returns the canonical env-style keys that a decoder
// would need to Resolve() for a given struct, using mapstructure tags when present.
// Example: prefix "GRPC_" and field Port `mapstructure:"port"` -> "GRPC_PORT".
func expectedKeysFromStructTags(prefix string, dst any) []string {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer {
		// accept struct by value too
		if v.Kind() == reflect.Struct {
			// make addressable copy
			tmp := reflect.New(v.Type())
			tmp.Elem().Set(v)
			v = tmp
		} else {
			return nil
		}
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil
	}

	pfx := normPrefix(prefix)
	out := make([]string, 0, v.NumField())
	seen := make(map[string]struct{})

	var walk func(reflect.Type, string)
	walk = func(t reflect.Type, curPrefix string) {
		// avoid nil pointer deref on time.Duration et al; treat as leaf
		if t.Kind() != reflect.Struct || isWellKnownLeaf(t) {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			// skip unexported
			if f.PkgPath != "" {
				continue
			}

			tag := f.Tag.Get("mapstructure")
			name, opts := parseMapstructureTag(tag)

			// skip explicitly ignored
			if name == "-" {
				continue
			}

			// default name from field if tag empty
			if name == "" {
				name = toUpperSnake(f.Name)
			} else {
				name = toUpperSnake(name)
			}

			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}

			// squash flattens nested struct fields into current level
			if opts["squash"] && ft.Kind() == reflect.Struct && !isWellKnownLeaf(ft) {
				walk(ft, curPrefix) // same prefix
				continue
			}

			// nested struct without squash: add another segment to prefix
			if ft.Kind() == reflect.Struct && !isWellKnownLeaf(ft) {
				walk(ft, curPrefix+name+"_")
				continue
			}

			key := curPrefix + name
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				out = append(out, key)
			}
		}
	}

	walk(v.Type(), pfx)
	return out
}

func normPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, ".", "_")
	p = strings.ReplaceAll(p, "-", "_")
	p = strings.ToUpper(p)
	// ensure trailing underscore for clean concatenation
	if !strings.HasSuffix(p, "_") {
		p += "_"
	}
	return p
}

func parseMapstructureTag(tag string) (name string, opts map[string]bool) {
	opts = make(map[string]bool)
	if tag == "" {
		return "", opts
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		if p != "" {
			opts[p] = true
		}
	}
	return name, opts
}

func toUpperSnake(s string) string {
	if s == "" {
		return ""
	}

	// Fast path: already snake_case (lowercase with underscores)
	if strings.ContainsAny(s, "_") && strings.ToLower(s) == s {
		return strings.ToUpper(s)
	}

	var b strings.Builder
	b.Grow(len(s) + 8) // reserve space for underscores

	for i, r := range s {
		// Replace delimiters with underscores
		if r == '-' || r == '.' {
			b.WriteByte('_')
			continue
		}

		// Insert underscore before uppercase if:
		// - not first char
		// - current is uppercase
		// - next char exists and is lowercase (handles "HTTPServer" -> "HTTP_Server")
		if i > 0 && unicode.IsUpper(r) {
			if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				b.WriteByte('_')
			}
		}

		b.WriteRune(unicode.ToUpper(r))
	}

	return b.String()
}

// Treat common types as leaves to avoid recursing into their internals.
func isWellKnownLeaf(t reflect.Type) bool {
	// time.Duration, etc.: compare by string name to avoid import
	if t.PkgPath() == "time" && t.Name() == "Duration" {
		return true
	}
	kind := t.Kind()
	return kind != reflect.Struct
}

// DecodePrefixInto collects all keys that start with prefix (e.g., "GRPC_" or "grpc")
// from r, strips the prefix, normalizes field names, and decodes into dst using tags.
func DecodePrefixInto(r resolver.PropertyResolver, prefix string, dst any) error {
	// Try to enumerate in one pass if the resolver supports it.
	var raw map[string]string
	var basePrefix string
	if e, ok := r.(enumerable); ok {
		raw = e.Entries()
		basePrefix = e.Prefix()
	} else {
		// Fallback: when we can't enumerate, we still need a list of expected keys.
		// Easiest: derive them from struct tags (mapstructure) and Resolve() one by one.
		want := expectedKeysFromStructTags(prefix, dst)
		m := make(map[string]string, len(want))
		for _, k := range want {
			if v, ok := r.Resolve(k); ok {
				m[k] = v
			}
		}
		raw = m
	}

	// Normalize prefix and collect the subsection.
	pfx := base.Normalize(prefix)
	if basePrefix != "" {
		pfx = base.Normalize(basePrefix) + pfx
	}
	section := make(map[string]any)

	for k, v := range raw {
		kk := base.Normalize(k)
		if !strings.HasPrefix(kk, pfx) {
			continue
		}
		// Strip prefix: GRPC_PORT -> PORT
		name := strings.TrimPrefix(kk, pfx)
		// Turn into tag-ish key: PORT -> port ; MAX_MESSAGE_SIZE -> max_message_size
		name = strings.ToLower(strings.ReplaceAll(name, "__", "_"))
		section[name] = v
	}

	// If no keys matched the prefix, don't decode (preserves defaults in dst)
	if len(section) == 0 {
		return nil
	}

	return decodeSectionInto(dst, section)
}

// --- Decode hooks  ---

func StringToDurationHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t != reflect.TypeOf(time.Duration(0)) {
			return data, nil
		}
		d, err := time.ParseDuration(data.(string))
		if err != nil {
			return nil, err
		}
		return d, nil
	}
}

func StringToBoolHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t.Kind() != reflect.Bool {
			return data, nil
		}
		s := strings.TrimSpace(strings.ToLower(data.(string)))
		switch s {
		case "1", "t", "true", "y", "yes", "on":
			return true, nil
		case "0", "f", "false", "n", "no", "off":
			return false, nil
		default:
			return nil, &mapstructure.Error{Errors: []string{"invalid bool: " + s}}
		}
	}
}

func StringToIntHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || (t.Kind() != reflect.Int && t.Kind() != reflect.Int64 && t.Kind() != reflect.Int32) {
			return data, nil
		}
		s := strings.TrimSpace(data.(string))
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		// mapstructure handles assignability to exact int sizes
		return i, nil
	}
}

func CSVToStringSliceHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t.Kind() != reflect.Slice || t.Elem().Kind() != reflect.String {
			return data, nil
		}
		s := strings.TrimSpace(data.(string))
		if s == "" {
			return []string(nil), nil
		}
		parts := strings.Split(s, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts, nil
	}
}

func StringToPortHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || t.Kind() != reflect.Uint16 {
			return data, nil
		}

		s := strings.TrimSpace(data.(string))
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		return base.Port(i), nil
	}
}

func StringToByteHook() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String || (t != reflect.TypeOf(base.Byte(0))) {
			return data, nil
		}
		s := strings.TrimSpace(strings.ToUpper(data.(string)))
		// Accept plain decimals and a few suffixes.
		mult := uint64(1)
		switch {
		case strings.HasSuffix(s, "KB"):
			mult, s = 1_000, strings.TrimSuffix(s, "KB")
		case strings.HasSuffix(s, "MB"):
			mult, s = 1_000_000, strings.TrimSuffix(s, "MB")
		case strings.HasSuffix(s, "GB"):
			mult, s = 1_000_000_000, strings.TrimSuffix(s, "GB")
		}
		i, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, err
		}
		return base.Byte(i * mult), nil
	}
}
