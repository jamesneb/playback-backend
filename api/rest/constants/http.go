package constants

import "time"

// Type aliases for enhanced type safety and semantic clarity
type (
	HTTPStatusCode  int
	HeaderName      string
	ContentType     string
	HTTPMethod      string
	TimeoutDuration time.Duration
)

// HTTP Status Code constants
const (
	StatusOK                  HTTPStatusCode = 200
	StatusNoContent           HTTPStatusCode = 204
	StatusInternalServerError HTTPStatusCode = 500
	StatusServiceUnavailable  HTTPStatusCode = 503
)

// HTTP Method constants
const (
	MethodOptions HTTPMethod = "OPTIONS"
	MethodGet     HTTPMethod = "GET"
	MethodPost    HTTPMethod = "POST"
)

// HTTP Header constants
const (
	HeaderXSSProtection         HeaderName = "X-XSS-Protection"
	HeaderContentTypeOptions    HeaderName = "X-Content-Type-Options"
	HeaderFrameOptions          HeaderName = "X-Frame-Options"
	HeaderReferrerPolicy        HeaderName = "Referrer-Policy"
	HeaderContentSecurityPolicy HeaderName = "Content-Security-Policy"
	HeaderHSTS                  HeaderName = "Strict-Transport-Security"
	HeaderPermissionsPolicy     HeaderName = "Permissions-Policy"
	HeaderCrossDomainPolicies   HeaderName = "X-Permitted-Cross-Domain-Policies"
	HeaderCOEP                  HeaderName = "Cross-Origin-Embedder-Policy"
	HeaderCOOP                  HeaderName = "Cross-Origin-Opener-Policy"
	HeaderCORP                  HeaderName = "Cross-Origin-Resource-Policy"
	HeaderXForwardedProto       HeaderName = "X-Forwarded-Proto"
	HeaderXForwardedSSL         HeaderName = "X-Forwarded-Ssl"
	HeaderXURLScheme            HeaderName = "X-Url-Scheme"
	HeaderAcceptEncoding        HeaderName = "Accept-Encoding"
	HeaderContentType           HeaderName = "Content-Type"
	HeaderContentEncoding       HeaderName = "Content-Encoding"
	HeaderVary                  HeaderName = "Vary"
)

// Header Value constants
const (
	XSSProtectionValue       = "1; mode=block"
	ContentTypeOptionsValue  = "nosniff"
	FrameOptionsDeny         = "DENY"
	ReferrerPolicyValue      = "strict-origin-when-cross-origin"
	HSTSValue                = "max-age=31536000; includeSubDomains; preload"
	PermissionsPolicyValue   = "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=()"
	CrossDomainPoliciesValue = "none"
	COEPValue                = "require-corp"
	COOPValue                = "same-origin"
	CORPValue                = "same-origin"

	// Protocol values
	ProtoHTTPS       = "https"
	SSLOnValue       = "on"
	EncodingGzip     = "gzip"
	TruncationSuffix = "..."
)

// Content Type constants
const (
	ContentTypeTextPlain         ContentType = "text/plain"
	ContentTypeApplicationJSON   ContentType = "application/json"
	ContentTypePrometheusMetrics ContentType = "text/plain; version=0.0.4; charset=utf-8"
	ContentTypeImagePrefix                   = "image/"
	ContentTypeVideoPrefix                   = "video/"
)

// Timeout constants
const (
	RequestTimeout     = 30 * time.Second
	HealthCheckTimeout = 5 * time.Second
)
