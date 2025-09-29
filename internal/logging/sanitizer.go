package logging

import (
	"crypto/sha256"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	// Regex patterns for sanitizing sensitive data
	sensitiveHeaderPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)password`),
		regexp.MustCompile(`(?i)token`),
		regexp.MustCompile(`(?i)secret`),
		regexp.MustCompile(`(?i)auth`),
		regexp.MustCompile(`(?i)key`),
		regexp.MustCompile(`(?i)session`),
		regexp.MustCompile(`(?i)bearer`),
	}

	// User agent patterns that should be anonymized
	sensitiveUserAgentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)email`),
		regexp.MustCompile(`(?i)user`),
		regexp.MustCompile(`(?i)login`),
		regexp.MustCompile(`(?i)account`),
	}
)

// SanitizeClientIP anonymizes client IP addresses while preserving network information
func SanitizeClientIP(clientIP string) string {
	if clientIP == "" {
		return "unknown"
	}

	// Parse the IP address
	ip := net.ParseIP(clientIP)
	if ip == nil {
		// If it's not a valid IP, hash it
		return hashString(clientIP)[:8]
	}

	// For IPv4, mask the last octet
	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.xxx", ipv4[0], ipv4[1], ipv4[2])
	}

	// For IPv6, mask the last 64 bits
	if ip.To16() != nil {
		ipv6 := ip.To16()
		return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x:xxxx:xxxx:xxxx:xxxx",
			ipv6[0], ipv6[1], ipv6[2], ipv6[3], ipv6[4], ipv6[5], ipv6[6], ipv6[7])
	}

	// Fallback: hash the IP
	return hashString(clientIP)[:12]
}

// SanitizeUserAgent removes or masks potentially sensitive information from User-Agent headers
func SanitizeUserAgent(userAgent string) string {
	if userAgent == "" {
		return "unknown"
	}

	// Check if the user agent contains sensitive patterns
	for _, pattern := range sensitiveUserAgentPatterns {
		if pattern.MatchString(userAgent) {
			return hashString(userAgent)[:16]
		}
	}

	// Remove version numbers and specific build information while keeping browser/OS info
	sanitized := userAgent

	// Replace version numbers with X.X.X
	versionRegex := regexp.MustCompile(`\d+\.\d+(\.\d+)*`)
	sanitized = versionRegex.ReplaceAllString(sanitized, "X.X.X")

	// Replace build numbers and specific identifiers
	buildRegex := regexp.MustCompile(`\b[A-Za-z0-9]{8,}\b`)
	sanitized = buildRegex.ReplaceAllString(sanitized, "XXXXXX")

	// Limit length to prevent log bloat
	if len(sanitized) > 100 {
		return sanitized[:97] + "..."
	}

	return sanitized
}

// SanitizeHeaderName checks if a header name contains sensitive information
func SanitizeHeaderName(headerName string) bool {
	lowerName := strings.ToLower(headerName)
	for _, pattern := range sensitiveHeaderPatterns {
		if pattern.MatchString(lowerName) {
			return true
		}
	}
	return false
}

// SanitizeDataSize bucketizes data sizes to prevent exact size leakage
func SanitizeDataSize(size int) string {
	switch {
	case size == 0:
		return "empty"
	case size < 1024:
		return "small" // < 1KB
	case size < 10*1024:
		return "medium" // 1KB - 10KB
	case size < 100*1024:
		return "large" // 10KB - 100KB
	case size < 1024*1024:
		return "xl" // 100KB - 1MB
	case size < 10*1024*1024:
		return "xxl" // 1MB - 10MB
	default:
		return "huge" // > 10MB
	}
}

// SanitizePath removes sensitive information from URL paths while preserving structure
func SanitizePath(path string) string {
	if path == "" {
		return "/"
	}

	// Replace UUIDs, IDs, tokens with placeholder
	uuidRegex := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	sanitized := uuidRegex.ReplaceAllString(path, "{uuid}")

	// Replace numeric IDs
	idRegex := regexp.MustCompile(`/\d+(/|$)`)
	sanitized = idRegex.ReplaceAllString(sanitized, "/{id}$1")

	// Replace hex tokens/IDs
	hexRegex := regexp.MustCompile(`/[0-9a-fA-F]{16,}(/|$)`)
	sanitized = hexRegex.ReplaceAllString(sanitized, "/{token}$1")

	// Limit length to prevent log bloat
	if len(sanitized) > 200 {
		return sanitized[:197] + "..."
	}

	return sanitized
}

// hashString creates a consistent hash of a string for anonymization
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// TenantID sanitizes tenant identifiers by preserving structure but anonymizing content
func SanitizeTenantID(tenantID string) string {
	if tenantID == "" || tenantID == "default" || tenantID == "unknown" {
		return tenantID
	}

	// Hash the tenant ID but keep a consistent short prefix for debugging
	hashed := hashString(tenantID)
	return "tenant_" + hashed[:8]
}

// ServiceName sanitizes service names by preserving common service patterns
func SanitizeServiceName(serviceName string) string {
	if serviceName == "" || serviceName == "unknown" {
		return serviceName
	}

	// Keep common service prefixes but anonymize the rest
	commonPrefixes := []string{"api-", "web-", "worker-", "service-", "app-"}

	for _, prefix := range commonPrefixes {
		if strings.HasPrefix(serviceName, prefix) {
			remainder := strings.TrimPrefix(serviceName, prefix)
			if len(remainder) > 0 {
				return prefix + hashString(remainder)[:6]
			}
		}
	}

	// If no common prefix, hash the entire name
	return "svc_" + hashString(serviceName)[:8]
}

// TraceID sanitizes trace IDs by showing structure but anonymizing content
func SanitizeTraceID(traceID string) string {
	if traceID == "" {
		return "empty"
	}

	// Keep length information and a small prefix for debugging
	if len(traceID) > 8 {
		return traceID[:4] + "****" + fmt.Sprintf("(%d)", len(traceID))
	}

	return "****" + fmt.Sprintf("(%d)", len(traceID))
}
