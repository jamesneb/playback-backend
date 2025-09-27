package constants

// Type aliases for CORS
type CORSHeaderValue string

// CORS Headers
const (
	HeaderAccessControlAllowOrigin      HeaderName = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowCredentials HeaderName = "Access-Control-Allow-Credentials"
	HeaderAccessControlAllowMethods     HeaderName = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     HeaderName = "Access-Control-Allow-Headers"
	HeaderAccessControlMaxAge           HeaderName = "Access-Control-Max-Age"
)

// CORS Values
const (
	CORSWildcardOrigin = "*"
	CORSMaxAgeSeconds  = "86400"
)

// CORS Messages
const (
	CORSNoOriginsWarning = "CORS enabled but no allowed origins configured"
)
