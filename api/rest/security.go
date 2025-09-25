package rest

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// securityHeadersMiddleware adds comprehensive security headers
func securityHeadersMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent XSS attacks
		c.Header(string(HEADER_XSS_PROTECTION), XSS_PROTECTION_VALUE)

		// Prevent MIME type sniffing
		c.Header(string(HEADER_CONTENT_TYPE_OPTIONS), CONTENT_TYPE_OPTIONS_VALUE)

		// Prevent clickjacking
		c.Header(string(HEADER_FRAME_OPTIONS), FRAME_OPTIONS_DENY)

		// Referrer policy
		c.Header(string(HEADER_REFERRER_POLICY), REFERRER_POLICY_VALUE)

		// Enhanced Content Security Policy
		csp := buildContentSecurityPolicy(cfg, c)
		c.Header(string(HEADER_CONTENT_SECURITY_POLICY), csp)

		// HSTS (only for HTTPS) with preload
		if isHTTPS(c) {
			c.Header(string(HEADER_HSTS), HSTS_VALUE)
		}

		// Enhanced permissions policy
		c.Header(string(HEADER_PERMISSIONS_POLICY), PERMISSIONS_POLICY_VALUE)

		// Additional security headers
		c.Header(string(HEADER_CROSS_DOMAIN_POLICIES), CROSS_DOMAIN_POLICIES_VALUE)
		c.Header(string(HEADER_COEP), COEP_VALUE)
		c.Header(string(HEADER_COOP), COOP_VALUE)
		c.Header(string(HEADER_CORP), CORP_VALUE)

		c.Next()
	}
}

// buildContentSecurityPolicy creates context-aware CSP
func buildContentSecurityPolicy(cfg *config.Config, c *gin.Context) string {
	baseCSP := string(CSP_DEFAULT_SRC_SELF)

	// Script sources
	scriptSrc := string(CSP_SCRIPT_SRC_SELF)

	// Style sources (allow inline for Swagger UI)
	styleSrc := string(CSP_STYLE_SRC_UNSAFE_INLINE)

	// Image sources
	imgSrc := string(CSP_IMG_SRC_DATA_HTTPS)

	// Font sources
	fontSrc := string(CSP_FONT_SRC_SELF)

	// Connect sources (API endpoints)
	connectSrc := string(CSP_CONNECT_SRC_SELF)

	// Special handling for Swagger UI
	if strings.Contains(c.Request.URL.Path, string(SWAGGER_PATH_SEGMENT)) {
		scriptSrc += string(CSP_SCRIPT_SRC_UNSAFE) // Swagger needs these
		connectSrc += string(CSP_CONNECT_SRC_HTTPS) // Swagger may need external connections
	}

	// Object and embed restrictions
	objectSrc := string(CSP_OBJECT_SRC_NONE)
	baseSrc := string(CSP_BASE_URI_SELF)

	// Form action restrictions
	formAction := string(CSP_FORM_ACTION_SELF)

	return fmt.Sprintf("%s; %s; %s; %s; %s; %s; %s; %s; %s",
		baseCSP, scriptSrc, styleSrc, imgSrc, fontSrc,
		connectSrc, objectSrc, baseSrc, formAction)
}

// isHTTPS determines if the request is over HTTPS
func isHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil ||
		c.Request.Header.Get(string(HEADER_X_FORWARDED_PROTO)) == PROTO_HTTPS ||
		c.Request.Header.Get(string(HEADER_X_FORWARDED_SSL)) == SSL_ON_VALUE ||
		c.Request.Header.Get(string(HEADER_X_URL_SCHEME)) == PROTO_HTTPS
}