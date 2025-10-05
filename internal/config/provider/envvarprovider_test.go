// Package provider tests environment variable configuration provider
package provider

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// getEnvVarProvider is a test helper that creates an EnvVarProvider with the given prefix
func getEnvVarProvider(prefix string) *EnvVarProvider {
	return &EnvVarProvider{Prefix: prefix}
}

func ExampleEnvVarProvider() {
	// Create an Environment Variable Provider
	// that filters env vars to those with "APP_" prefix
	provider := getEnvVarProvider("APP_")
	// Load environment variables to map
	// providers should not mutate their maps after assignment
	envVars, err := provider.Load(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Provider name: ", provider.Name())
	fmt.Println("Loaded keys: ", len(envVars))
	// Output:
	// Provider name:  APP_env
	// Loaded keys:  0
}

func ExampleEnvVarProvider_Name() {
	// Create an Environment Variable Provider
	// with APP_ prefix
	provider := getEnvVarProvider("APP_")
	fmt.Println("Provider name: ", provider.Name())

	// Create an Environment Variable Provider
	// with no prefix
	provider = getEnvVarProvider("")
	fmt.Println("Provider name #2: ", provider.Name())

	// Output:
	// Provider name:  APP_env
	// Provider name #2:  env
}

// TestEnvVarProvider_Name tests the Name() method which returns a sanitized provider name
// by concatenating the prefix with the base provider name. Tests verify:
// - Prefix concatenation works correctly
// - Empty prefix returns just the base name
// - Long prefixes are truncated to MAX_PREFIX_CHARS
// - Special characters are stripped from prefix
func TestEnvVarProvider_Name(t *testing.T) {
	longPrefix := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"with_prefix", "APP_", "APP_" + ENV_VAR_PROVIDER_NAME},
		{"without_prefix", "", ENV_VAR_PROVIDER_NAME},
		{"other_prefix", "OTHER_", "OTHER_" + ENV_VAR_PROVIDER_NAME},
		{"too_long_prefix", longPrefix, longPrefix[:32] + ENV_VAR_PROVIDER_NAME},
		{"special_char_prefix", "/%$!--", ENV_VAR_PROVIDER_NAME},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := getEnvVarProvider(tt.prefix)
			got := provider.Name()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEnvProvider_Load tests the Load() method which reads environment variables from the OS.
// Uses table-driven tests with custom setup and check functions per test case. Tests verify:
// - Empty prefix loads all environment variables
// - Prefix filtering correctly includes/excludes variables
// - Case-insensitive prefix matching works
// - Empty values are preserved
// - Whitespace-only values are preserved (no trimming)
// - Keys are normalized to uppercase
// - Non-matching prefixes return empty maps
// - Multiple '=' characters in values are handled (everything after first '=' is the value)
// - Special characters like newlines in values are preserved
func TestEnvProvider_Load(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		setupEnv func(*testing.T)
		check    func(*testing.T, map[string]string, error)
	}{
		{
			// Verify that with no prefix filtering, Load returns at least some env vars
			name:     "empty_prefix_loads_env_vars",
			prefix:   "",
			setupEnv: func(t *testing.T) {},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Greater(t, len(cfg), 0)
			},
		},

		{
			// Verify prefix filtering: only vars starting with prefix are included
			name:   "prefix_filters_env_vars",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TESTAPP_FOO", "bar")
				t.Setenv("OTHERAPP_BAZ", "qux")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FOO")
				assert.Equal(t, "bar", cfg["TESTAPP_FOO"])
				assert.NotContains(t, cfg, "OTHERAPP_BAZ")
			},
		},

		{
			// Verify case-insensitive matching: mixed-case key gets uppercased and matched
			name:   "matches_prefix_ignores_case",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TeStApP_FOO", "bar")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FOO")
				assert.Equal(t, "bar", cfg["TESTAPP_FOO"])
			},
		},

		{
			// Verify empty string values are loaded and preserved
			name:   "loads_empty_value",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TESTAPP_FOO", "")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FOO")
				assert.Equal(t, "", cfg["TESTAPP_FOO"])
			},
		},
		{
			// Verify whitespace-only values are NOT trimmed (Load() preserves raw values)
			name:   "preserves_whitespace_only_values",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TESTAPP_FOO", " ")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FOO")
				assert.Equal(t, " ", cfg["TESTAPP_FOO"])
			},
		},

		{
			// Verify lowercase keys are normalized to uppercase in the returned map
			name:   "keys_normalized_to_upper",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("testapp_foo", "bar")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FOO")
				assert.NotContains(t, cfg, "testapp_foo")
			},
		},

		{
			// Verify that a prefix with no matches returns an empty map
			name:     "no_matches_with_prefix_returns_empty_map",
			prefix:   "UNITTEST_NOMATCH_",
			setupEnv: func(t *testing.T) {},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Empty(t, cfg)
			},
		},

		{
			// Verify values containing '=' characters are handled correctly
			// (everything after the FIRST '=' is treated as the value)
			name:   "loads_everything_after_first_equal_as_value",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TESTAPP_URL", "postgres://user:pass@host/db?foo=bar")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_URL")
				assert.Equal(t, "postgres://user:pass@host/db?foo=bar", cfg["TESTAPP_URL"])
			},
		},
		{
			// Verify special characters like newlines in values are preserved
			name:   "newlines_in_value_preserved",
			prefix: "TESTAPP_",
			setupEnv: func(t *testing.T) {
				t.Setenv("TESTAPP_FILE", "/path/withnewline\n/file")
			},
			check: func(t *testing.T, cfg map[string]string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, cfg, "TESTAPP_FILE")
				assert.Equal(t, "/path/withnewline\n/file", cfg["TESTAPP_FILE"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			provider := getEnvVarProvider(tt.prefix)
			cfg, err := provider.Load(context.Background())
			tt.check(t, cfg, err)
		})
	}
}
