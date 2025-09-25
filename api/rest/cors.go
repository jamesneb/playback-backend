package rest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// createCorsMiddleware creates CORS middleware with complete origin matching and performance optimizations
func createCorsMiddleware(corsConfig config.CORSConfig) (gin.HandlerFunc, error) {
	if len(corsConfig.AllowedOrigins) == 0 {
		return nil, errors.New("CORS enabled but no allowed origins specified")
	}

	// Pre-compute header values to avoid string concatenation in hot path
	var methodsHeader string
	if len(corsConfig.AllowedMethods) > 0 {
		methodsHeader = strings.Join(corsConfig.AllowedMethods, ", ")
	}

	var headersHeader string
	if len(corsConfig.AllowedHeaders) > 0 {
		headersHeader = strings.Join(corsConfig.AllowedHeaders, ", ")
	}

	// Pre-process origins for efficient matching
	allowAllOrigins := len(corsConfig.AllowedOrigins) == 1 && corsConfig.AllowedOrigins[0] == CORS_WILDCARD_ORIGIN

	// Create origin lookup structures for optimal performance
	exactOrigins := make(map[string]bool)
	var patternOrigins []originPattern

	for _, origin := range corsConfig.AllowedOrigins {
		if origin == CORS_WILDCARD_ORIGIN {
			continue // handled by allowAllOrigins flag
		}
		if strings.Contains(origin, "*") || strings.Contains(origin, "?") || strings.Contains(origin, "[") {
			// Pattern origin - compile for matching
			compiled, err := compileOriginPattern(origin)
			if err != nil {
				return nil, fmt.Errorf("invalid origin pattern '%s': %w", origin, err)
			}
			patternOrigins = append(patternOrigins, compiled)
		} else {
			// Exact match origin
			exactOrigins[origin] = true
		}
	}

	return func(c *gin.Context) {
		requestOrigin := c.Request.Header.Get("Origin")

		// Handle origin matching
		if allowAllOrigins {
			c.Header(string(HEADER_ACCESS_CONTROL_ALLOW_ORIGIN), CORS_WILDCARD_ORIGIN)
		} else if requestOrigin != "" {
			allowed := false

			// Check exact matches first (O(1) lookup)
			if exactOrigins[requestOrigin] {
				allowed = true
			} else {
				// Check pattern matches
				for _, pattern := range patternOrigins {
					if pattern.matches(requestOrigin) {
						allowed = true
						break
					}
				}
			}

			if allowed {
				c.Header(string(HEADER_ACCESS_CONTROL_ALLOW_ORIGIN), requestOrigin)
				c.Header(string(HEADER_ACCESS_CONTROL_ALLOW_CREDENTIALS), "true")
			} else {
				logger.Warn("CORS origin rejected",
					zap.String("origin", requestOrigin),
					zap.Strings("allowed_origins", corsConfig.AllowedOrigins))
			}
		}

		// Pre-computed headers for performance
		if methodsHeader != "" {
			c.Header(string(HEADER_ACCESS_CONTROL_ALLOW_METHODS), methodsHeader)
		}
		if headersHeader != "" {
			c.Header(string(HEADER_ACCESS_CONTROL_ALLOW_HEADERS), headersHeader)
		}

		// Additional CORS headers for robustness
		c.Header(string(HEADER_ACCESS_CONTROL_MAX_AGE), CORS_MAX_AGE_SECONDS) // 24 hours preflight cache

		// Handle preflight requests
		if c.Request.Method == string(METHOD_OPTIONS) {
			c.AbortWithStatus(int(StatusNoContent))
			return
		}

		c.Next()
	}, nil
}

// originPattern represents a compiled origin pattern for efficient matching
type originPattern struct {
	original string
	matcher  func(string) bool
}

// matches tests if an origin matches this pattern
func (p originPattern) matches(origin string) bool {
	return p.matcher(origin)
}

// compileOriginPattern compiles an origin pattern into an efficient matcher
func compileOriginPattern(pattern string) (originPattern, error) {
	if pattern == "" {
		return originPattern{}, errors.New("empty pattern")
	}

	// Create matcher function based on pattern complexity
	matcher := createGlobMatcher(pattern)

	return originPattern{
		original: pattern,
		matcher:  matcher,
	}, nil
}

// createGlobMatcher creates a complete glob pattern matcher
func createGlobMatcher(pattern string) func(string) bool {
	return func(text string) bool {
		return matchGlob(pattern, text)
	}
}

// matchGlob implements complete glob pattern matching
// Supports: * (any chars), ? (single char), [abc] (char class), [a-z] (ranges)
func matchGlob(pattern, text string) bool {
	return matchGlobRecursive(pattern, text, 0, 0)
}

func matchGlobRecursive(pattern, text string, pi, ti int) bool {
	for pi < len(pattern) {
		switch pattern[pi] {
		case '*':
			// Handle consecutive stars as single star
			for pi < len(pattern) && pattern[pi] == '*' {
				pi++
			}
			// If star is at end, match succeeds
			if pi == len(pattern) {
				return true
			}
			// Try matching rest of pattern at each possible position
			for ti <= len(text) {
				if matchGlobRecursive(pattern, text, pi, ti) {
					return true
				}
				ti++
			}
			return false
		case '?':
			// Single character wildcard
			if ti >= len(text) {
				return false
			}
			pi++
			ti++
		case '[':
			// Character class
			if ti >= len(text) {
				return false
			}
			pi++ // skip '['
			matched := false
			negated := false
			if pi < len(pattern) && pattern[pi] == '^' {
				negated = true
				pi++
			}

			for pi < len(pattern) && pattern[pi] != ']' {
				if pi+2 < len(pattern) && pattern[pi+1] == '-' {
					// Range like 'a-z'
					if text[ti] >= pattern[pi] && text[ti] <= pattern[pi+2] {
						matched = true
					}
					pi += 3
				} else {
					// Single character
					if text[ti] == pattern[pi] {
						matched = true
					}
					pi++
				}
			}

			if pi >= len(pattern) {
				return false // unclosed bracket
			}
			pi++ // skip ']'

			if negated {
				matched = !matched
			}
			if !matched {
				return false
			}
			ti++
		case '\\':
			// Escape character
			pi++
			if pi >= len(pattern) {
				return false
			}
			if ti >= len(text) || text[ti] != pattern[pi] {
				return false
			}
			pi++
			ti++
		default:
			// Literal character
			if ti >= len(text) || text[ti] != pattern[pi] {
				return false
			}
			pi++
			ti++
		}
	}

	// Pattern consumed, text should also be consumed
	return ti == len(text)
}