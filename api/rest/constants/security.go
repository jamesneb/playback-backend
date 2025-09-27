package constants

// Type aliases for security
type CSPDirective string

// CSP Directive constants
const (
	CSPDefaultSrcSelf       CSPDirective = "default-src 'self'"
	CSPScriptSrcSelf        CSPDirective = "script-src 'self'"
	CSPScriptSrcUnsafe      CSPDirective = " 'unsafe-inline' 'unsafe-eval'"
	CSPStyleSrcUnsafeInline CSPDirective = "style-src 'self' 'unsafe-inline'"
	CSPImgSrcDataHTTPS      CSPDirective = "img-src 'self' data: https:"
	CSPFontSrcSelf          CSPDirective = "font-src 'self'"
	CSPConnectSrcSelf       CSPDirective = "connect-src 'self'"
	CSPConnectSrcHTTPS      CSPDirective = " https:"
	CSPObjectSrcNone        CSPDirective = "object-src 'none'"
	CSPBaseURISelf          CSPDirective = "base-uri 'self'"
	CSPFormActionSelf       CSPDirective = "form-action 'self'"
)
