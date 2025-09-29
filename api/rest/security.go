package rest

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// securityHeadersMiddleware adds comprehensive security headers
func securityHeadersMiddleware(cfg *config.ConsolidatedConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent XSS attacks
		c.Header(string(constants.HeaderXSSProtection), constants.XSSProtectionValue)

		// Prevent MIME type sniffing
		c.Header(string(constants.HeaderContentTypeOptions), constants.ContentTypeOptionsValue)

		// Prevent clickjacking
		c.Header(string(constants.HeaderFrameOptions), constants.FrameOptionsDeny)

		// Referrer policy
		c.Header(string(constants.HeaderReferrerPolicy), constants.ReferrerPolicyValue)

		// Enhanced Content Security Policy
		csp := buildContentSecurityPolicy(cfg, c)
		c.Header(string(constants.HeaderContentSecurityPolicy), csp)

		// HSTS (only for HTTPS) with preload
		if isHTTPS(c) {
			c.Header(string(constants.HeaderHSTS), constants.HSTSValue)
		}

		// Enhanced permissions policy
		c.Header(string(constants.HeaderPermissionsPolicy), constants.PermissionsPolicyValue)

		// Additional security headers
		c.Header(string(constants.HeaderCrossDomainPolicies), constants.CrossDomainPoliciesValue)
		c.Header(string(constants.HeaderCOEP), constants.COEPValue)
		c.Header(string(constants.HeaderCOOP), constants.COOPValue)
		c.Header(string(constants.HeaderCORP), constants.CORPValue)

		c.Next()
	}
}

// buildContentSecurityPolicy creates context-aware CSP
func buildContentSecurityPolicy(cfg *config.ConsolidatedConfig, c *gin.Context) string {
	baseCSP := string(constants.CSPDefaultSrcSelf)

	// Script sources
	scriptSrc := string(constants.CSPScriptSrcSelf)

	// Style sources (allow inline for Swagger UI)
	styleSrc := string(constants.CSPStyleSrcUnsafeInline)

	// Image sources
	imgSrc := string(constants.CSPImgSrcDataHTTPS)

	// Font sources
	fontSrc := string(constants.CSPFontSrcSelf)

	// Connect sources (API endpoints)
	connectSrc := string(constants.CSPConnectSrcSelf)

	// Special handling for Swagger UI
	if strings.Contains(c.Request.URL.Path, string(constants.SwaggerPathSegment)) {
		scriptSrc += string(constants.CSPScriptSrcUnsafe)  // Swagger needs these
		connectSrc += string(constants.CSPConnectSrcHTTPS) // Swagger may need external connections
	}

	// Object and embed restrictions
	objectSrc := string(constants.CSPObjectSrcNone)
	baseSrc := string(constants.CSPBaseURISelf)

	// Form action restrictions
	formAction := string(constants.CSPFormActionSelf)

	return fmt.Sprintf("%s; %s; %s; %s; %s; %s; %s; %s; %s",
		baseCSP, scriptSrc, styleSrc, imgSrc, fontSrc,
		connectSrc, objectSrc, baseSrc, formAction)
}

// isHTTPS determines if the request is over HTTPS
func isHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil ||
		c.Request.Header.Get(string(constants.HeaderXForwardedProto)) == constants.ProtoHTTPS ||
		c.Request.Header.Get(string(constants.HeaderXForwardedSSL)) == constants.SSLOnValue ||
		c.Request.Header.Get(string(constants.HeaderXURLScheme)) == constants.ProtoHTTPS
}
