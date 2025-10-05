// Package http defines the configuration for the HTTP/REST API server.
//
// # Overview
//
// This package provides comprehensive HTTP server configuration including:
//
//   - Server basics (host, port, mode)
//   - Timeouts (read, write, idle, shutdown)
//   - Size limits (request, header)
//   - API configuration (prefix, trusted proxies)
//   - CORS (Cross-Origin Resource Sharing)
//   - Rate limiting (requests per second, burst)
//   - TLS/SSL (certificates, versions)
//   - JWT authentication (secret, expiry, refresh)
//   - Performance (profiling, compression, keep-alive)
//   - Development tools (Swagger, debug mode)
//
// # Configuration Keys
//
// All settings use the HTTP_ prefix:
//
//	HTTP_HOST             - Server bind address (default: 0.0.0.0)
//	HTTP_PORT             - Server port (default: 8080, range: 1024-65535)
//	HTTP_MODE             - Server mode: debug|release|test (default: release)
//	HTTP_READ_TIMEOUT     - Read timeout (default: 30s, range: 1s-5m)
//	HTTP_WRITE_TIMEOUT    - Write timeout (default: 30s, range: 1s-5m)
//	HTTP_IDLE_TIMEOUT     - Idle connection timeout (default: 60s, range: 1s-5m)
//	HTTP_SHUTDOWN_TIMEOUT - Graceful shutdown timeout (default: 30s, range: 1s-5m)
//
// Size limits:
//
//	HTTP_MAX_REQUEST_SIZE - Maximum request body (default: 25MB, range: 1MB-100MB)
//	HTTP_MAX_HEADER_SIZE  - Maximum headers size (default: 1MB, range: 1KB-10MB)
//
// API configuration:
//
//	HTTP_API_PREFIX       - API route prefix (default: /api/v1)
//	HTTP_TRUSTED_PROXIES  - Comma-separated proxy IPs
//
// CORS configuration:
//
//	HTTP_ENABLE_CORS              - Enable CORS (default: true)
//	HTTP_CORS_ALLOWED_ORIGINS     - Allowed origins (default: *)
//	HTTP_CORS_ALLOWED_METHODS     - Allowed methods (default: GET,POST,PUT,DELETE,OPTIONS,HEAD,PATCH)
//	HTTP_CORS_ALLOWED_HEADERS     - Allowed headers (default: Origin,Content-Type,Accept,Authorization,X-Requested-With)
//	HTTP_CORS_EXPOSED_HEADERS     - Exposed headers (default: Content-Length)
//	HTTP_CORS_ALLOW_CREDENTIALS   - Allow credentials (default: false)
//	HTTP_CORS_MAX_AGE             - Preflight cache duration (default: 1h, range: 0s-24h)
//
// Rate limiting (set to 0 to disable):
//
//	HTTP_RATE_LIMIT_RPS   - Requests per second (default: 1000, range: 1-1000000)
//	HTTP_RATE_LIMIT_BURST - Burst capacity (default: 2000, max: 10x RPS)
//
// TLS/SSL:
//
//	HTTP_TLS_ENABLED      - Enable TLS (default: false)
//	HTTP_TLS_CERT_FILE    - Certificate file path (required if TLS enabled)
//	HTTP_TLS_KEY_FILE     - Private key file path (required if TLS enabled)
//	HTTP_TLS_CA_FILE      - CA certificate file (optional)
//	HTTP_TLS_MIN_VERSION  - Minimum TLS version: 1.0|1.1|1.2|1.3 (default: 1.2)
//	HTTP_TLS_MAX_VERSION  - Maximum TLS version (default: 1.3)
//
// JWT authentication:
//
//	HTTP_ENABLE_AUTH        - Enable JWT auth (default: false)
//	HTTP_JWT_SECRET         - Signing secret (required if auth enabled)
//	HTTP_JWT_EXPIRY         - Token lifetime (default: 24h, range: 1h-168h)
//	HTTP_JWT_REFRESH_WINDOW - Refresh window before expiry (default: 168h, range: 1h-720h)
//	HTTP_JWT_ISSUER         - Token issuer (default: playback-backend)
//	HTTP_JWT_AUDIENCE       - Token audience (default: playback-api)
//
// Performance:
//
//	HTTP_ENABLE_PROFILING      - Enable pprof endpoints (default: false)
//	HTTP_COMPRESSION_LEVEL     - gzip level 1-9 (default: 6)
//	HTTP_COMPRESSION_THRESHOLD - Min size to compress (default: 1KB, range: 1KB-1MB)
//	HTTP_KEEP_ALIVE            - Enable HTTP keep-alive (default: true)
//	HTTP_KEEP_ALIVE_TIMEOUT    - Keep-alive timeout (default: 1m, range: 10s-5m)
//
// Development:
//
//	HTTP_ENABLE_SWAGGER - Enable Swagger UI (default: false)
//	HTTP_SWAGGER_PATH   - Swagger endpoint path (default: /swagger)
//	HTTP_ENABLE_DEBUG   - Enable debug logging (default: false)
//
// # Example Usage
//
//	// Get HTTP config from manager
//	snapshot := mgr.Snapshot()
//	httpCfg := snapshot.HTTP
//
//	// Use configuration
//	server := &http.Server{
//	    Addr:              fmt.Sprintf("%s:%d", httpCfg.Host, httpCfg.Port),
//	    ReadTimeout:       httpCfg.ReadTimeout,
//	    WriteTimeout:      httpCfg.WriteTimeout,
//	    IdleTimeout:       httpCfg.IdleTimeout,
//	    MaxHeaderBytes:    int(httpCfg.MaxHeaderSize),
//	}
//
// # Validation
//
// The configuration is validated on load with:
//
//   - Port in valid range (1024-65535)
//   - All timeouts within bounds (1s to 5m)
//   - Size limits within bounds
//   - Rate limiting: burst must be 0 if RPS is 0
//   - Rate limiting: burst must be ≤ 10x RPS
//   - TLS: cert and key files required when enabled
//   - JWT: secret required when auth enabled
//   - CORS: max age within bounds when CORS enabled
//
// # Rate Limiting Details
//
// The rate limiter uses a token bucket algorithm:
//
//   - RPS defines token refill rate
//   - Burst defines bucket capacity
//   - Set both to 0 to disable rate limiting
//   - Burst must be at least as large as RPS
//   - Burst should not exceed 10x RPS for memory efficiency
//
// # TLS Configuration
//
// When TLS is enabled:
//
//   - Server will listen on HTTPS instead of HTTP
//   - CertFile and KeyFile are required
//   - CAFile is optional (for mutual TLS)
//   - MinVersion should be at least 1.2 for security
//   - MaxVersion can limit protocol versions
//
// # JWT Authentication
//
// When authentication is enabled:
//
//   - All protected endpoints require valid JWT
//   - Tokens are signed with JWTSecret
//   - Tokens expire after JWTExpiry duration
//   - Tokens can be refreshed within JWTRefreshWindow before expiry
//   - Issuer and Audience claims are validated
//
// # CORS Configuration
//
// CORS is enabled by default with permissive settings:
//
//   - AllowedOrigins: * (all origins)
//   - AllowedMethods: All common HTTP methods
//   - AllowedHeaders: Common headers including Authorization
//   - AllowCredentials: false (can't use with wildcard origin)
//
// For production, configure specific origins:
//
//	HTTP_CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
//	HTTP_CORS_ALLOW_CREDENTIALS=true
//
// # Performance Tuning
//
// For high-traffic scenarios:
//
//   - Increase RateLimitRPS and RateLimitBurst
//   - Enable compression for text responses (reduces bandwidth)
//   - Enable KeepAlive to reuse connections
//   - Tune timeouts based on response time requirements
//   - Consider increasing MaxRequestSize for large payloads
//
// For low-latency scenarios:
//
//   - Disable compression (saves CPU)
//   - Reduce timeout values
//   - Minimize MaxHeaderSize
package http
