// Package grpc defines the configuration for the gRPC server.
//
// # Overview
//
// This package provides comprehensive gRPC server configuration including:
//
//   - Server basics (port)
//   - Message size limits (send, receive)
//   - Connection management (timeouts)
//   - Rate limiting (requests per second, burst capacity)
//   - TLS/SSL (certificates, versions)
//   - Token authentication (secret-based auth)
//
// # Configuration Keys
//
// All settings use the GRPC_ prefix:
//
// Server basics:
//
//	GRPC_SERVER_PORT - Server port (default: 4317, range: 1-65535)
//
// Message size limits:
//
//	GRPC_MAX_RECEIVE_SIZE - Maximum message size to receive (default: 16MB, must be > 0)
//	GRPC_MAX_SEND_SIZE    - Maximum message size to send (default: 16MB, must be > 0)
//
// Connection management:
//
//	GRPC_MAX_CONNECTION_TIMEOUT - Connection timeout (default: 30s, range: 100ms-30s)
//
// Rate limiting (set to 0 to disable):
//
//	GRPC_MAX_REQUESTS_PER_SECOND    - Requests per second (default: 100, range: 1-200000)
//	GRPC_MAX_REQUEST_BURST_CAPACITY - Burst capacity (default: 200, range: 1-1000000, max: 2x RPS)
//
// TLS/SSL:
//
//	GRPC_TLS_ENABLED     - Enable TLS (default: false)
//	GRPC_TLS_CERT_FILE   - Certificate file path (required if TLS enabled)
//	GRPC_TLS_KEY_FILE    - Private key file path (required if TLS enabled)
//	GRPC_TLS_CA_FILE     - CA certificate file (optional, for mutual TLS)
//	GRPC_TLS_MIN_VERSION - Minimum TLS version: 1.0|1.1|1.2|1.3 (default: 1.2)
//	GRPC_TLS_MAX_VERSION - Maximum TLS version (default: 1.3)
//
// Token authentication:
//
//	GRPC_ENABLE_TOKEN_AUTH - Enable token-based auth (default: false)
//	GRPC_TOKEN_SECRET      - Token signing secret (required if auth enabled)
//
// # Example Usage
//
//	// Get gRPC config from manager
//	snapshot := mgr.Snapshot()
//	grpcCfg := snapshot.GRPC
//
//	// Create gRPC server with configuration
//	opts := []grpc.ServerOption{
//	    grpc.MaxRecvMsgSize(int(grpcCfg.MaxReceiveSize)),
//	    grpc.MaxSendMsgSize(int(grpcCfg.MaxSendSize)),
//	    grpc.ConnectionTimeout(grpcCfg.ConnectionTimeout),
//	}
//
//	// Add TLS if enabled
//	if grpcCfg.TLS.Enabled {
//	    creds, err := credentials.NewServerTLSFromFile(
//	        grpcCfg.TLS.CertFile,
//	        grpcCfg.TLS.KeyFile,
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    opts = append(opts, grpc.Creds(creds))
//	}
//
//	// Add rate limiting if enabled
//	if grpcCfg.RequestsPerSecond > 0 {
//	    limiter := rate.NewLimiter(
//	        rate.Limit(grpcCfg.RequestsPerSecond),
//	        grpcCfg.RequestBurstCapacity,
//	    )
//	    opts = append(opts, grpc.UnaryInterceptor(rateLimitInterceptor(limiter)))
//	}
//
//	// Create and start server
//	server := grpc.NewServer(opts...)
//	lis, _ := net.Listen("tcp", fmt.Sprintf(":%d", grpcCfg.Port))
//	server.Serve(lis)
//
// # Validation
//
// The configuration is validated on load with:
//
// Basic validation:
//
//   - Port in valid range (1-65535)
//   - MaxReceiveSize greater than 0
//   - MaxSendSize greater than 0
//   - ConnectionTimeout within bounds (100ms to 30s)
//
// Rate limiting validation:
//
//   - RequestsPerSecond in range (1-200,000) or 0 to disable
//   - When RPS is 0, RequestBurstCapacity must also be 0
//   - When RPS > 0, RequestBurstCapacity must be in range (1-1,000,000)
//   - RequestBurstCapacity must not exceed 2x RequestsPerSecond
//
// TLS validation (only when enabled):
//
//   - CertFile not empty
//   - KeyFile not empty
//   - CAFile optional (for mutual TLS)
//
// Token auth validation (only when enabled):
//
//   - TokenSecret not empty
//
// # Rate Limiting Details
//
// The rate limiter uses a token bucket algorithm:
//
//   - RequestsPerSecond defines token refill rate
//   - RequestBurstCapacity defines bucket capacity
//   - Set both to 0 to disable rate limiting
//   - Burst must be at least equal to RPS
//   - Burst should not exceed 2x RPS for memory efficiency
//
// Rate limiting prevents:
//
//   - Resource exhaustion from excessive requests
//   - Denial of service attacks
//   - Cascading failures in distributed systems
//   - Cost overruns in metered environments
//
// Example configurations:
//
//	# Moderate traffic: 100 RPS with 2x burst
//	GRPC_MAX_REQUESTS_PER_SECOND=100
//	GRPC_MAX_REQUEST_BURST_CAPACITY=200
//
//	# High traffic: 10,000 RPS with 2x burst
//	GRPC_MAX_REQUESTS_PER_SECOND=10000
//	GRPC_MAX_REQUEST_BURST_CAPACITY=20000
//
//	# Disable rate limiting
//	GRPC_MAX_REQUESTS_PER_SECOND=0
//	GRPC_MAX_REQUEST_BURST_CAPACITY=0
//
// # Message Size Limits
//
// Message size limits prevent memory exhaustion:
//
//   - MaxReceiveSize caps incoming message size
//   - MaxSendSize caps outgoing message size
//   - Default 16MB handles most use cases
//   - Increase for large payloads (logs, traces, metrics)
//   - Consider network and memory constraints
//
// Sizing recommendations:
//
//	# Small messages (metrics, simple RPCs)
//	GRPC_MAX_RECEIVE_SIZE=1048576    # 1MB
//	GRPC_MAX_SEND_SIZE=1048576       # 1MB
//
//	# Medium messages (traces, logs)
//	GRPC_MAX_RECEIVE_SIZE=16777216   # 16MB (default)
//	GRPC_MAX_SEND_SIZE=16777216      # 16MB (default)
//
//	# Large messages (bulk exports, file transfers)
//	GRPC_MAX_RECEIVE_SIZE=104857600  # 100MB
//	GRPC_MAX_SEND_SIZE=104857600     # 100MB
//
// # TLS Configuration
//
// When TLS is enabled:
//
//   - Server uses HTTPS instead of HTTP
//   - CertFile and KeyFile are required
//   - CAFile is optional (for mutual TLS)
//   - MinVersion should be at least 1.2 for security
//   - MaxVersion can limit protocol versions
//
// TLS best practices:
//
//   - Always enable TLS in production
//   - Use certificates from trusted CA
//   - Rotate certificates before expiry
//   - Use mutual TLS for service-to-service auth
//   - Disable TLS 1.0 and 1.1 (deprecated and insecure)
//
// Example TLS configuration:
//
//	# Enable TLS with certificates
//	GRPC_TLS_ENABLED=true
//	GRPC_TLS_CERT_FILE=/etc/certs/server.crt
//	GRPC_TLS_KEY_FILE=/etc/certs/server.key
//	GRPC_TLS_MIN_VERSION=1.2
//	GRPC_TLS_MAX_VERSION=1.3
//
//	# Add mutual TLS (client cert verification)
//	GRPC_TLS_CA_FILE=/etc/certs/ca.crt
//
// # Token Authentication
//
// When token authentication is enabled:
//
//   - All requests must include valid token
//   - Tokens are signed with TokenSecret
//   - Tokens should be passed in metadata
//   - Use for service-to-service authentication
//
// Token auth implementation:
//
//	// Server-side: verify token in interceptor
//	func authInterceptor(ctx context.Context, req interface{},
//	    info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
//	    md, ok := metadata.FromIncomingContext(ctx)
//	    if !ok {
//	        return nil, status.Errorf(codes.Unauthenticated, "no metadata")
//	    }
//	    tokens := md.Get("authorization")
//	    if len(tokens) == 0 {
//	        return nil, status.Errorf(codes.Unauthenticated, "no token")
//	    }
//	    if !verifyToken(tokens[0], cfg.TokenSecret) {
//	        return nil, status.Errorf(codes.Unauthenticated, "invalid token")
//	    }
//	    return handler(ctx, req)
//	}
//
//	// Client-side: add token to metadata
//	md := metadata.New(map[string]string{"authorization": token})
//	ctx := metadata.NewOutgoingContext(context.Background(), md)
//	client.Call(ctx, req)
//
// # Connection Timeout
//
// The connection timeout controls:
//
//   - How long to wait for client connections
//   - Prevents hanging on slow or dead clients
//   - Default 30s handles most network conditions
//   - Reduce for low-latency requirements
//   - Increase for high-latency networks
//
// Timeout recommendations:
//
//   - Local network: 5-10s
//   - Internet: 30s (default)
//   - Satellite/slow: 60s+
//
// # Port Selection
//
// Port 4317 is the standard OTLP gRPC port:
//
//   - Used by OpenTelemetry Protocol (OTLP)
//   - Widely recognized in observability ecosystem
//   - Avoids conflicts with common services
//   - Use different port if running multiple gRPC servers
//
// Common gRPC ports:
//
//   - 4317: OTLP gRPC (default)
//   - 9090: Prometheus (metrics)
//   - 14250: Jaeger gRPC
//   - 50051: Generic gRPC services
//
// # Performance Tuning
//
// For high-throughput scenarios:
//
//   - Increase RequestsPerSecond and RequestBurstCapacity
//   - Increase MaxReceiveSize and MaxSendSize for large messages
//   - Enable HTTP/2 settings tuning
//   - Consider connection pooling on client side
//   - Monitor CPU and memory usage
//
// For low-latency scenarios:
//
//   - Reduce ConnectionTimeout
//   - Use smaller message sizes
//   - Enable gRPC keepalive settings
//   - Use streaming RPCs for continuous data
//
// # Security Best Practices
//
// For production deployments:
//
//   - Always enable TLS
//   - Use token authentication or mutual TLS
//   - Set reasonable rate limits
//   - Limit message sizes to prevent DoS
//   - Keep certificates up to date
//   - Use strong token secrets (32+ bytes)
//   - Rotate secrets periodically
//   - Monitor for authentication failures
//
// # Complete Example
//
// Production-ready gRPC server configuration:
//
//	# Server basics
//	GRPC_SERVER_PORT=4317
//
//	# Message limits
//	GRPC_MAX_RECEIVE_SIZE=16777216  # 16MB
//	GRPC_MAX_SEND_SIZE=16777216     # 16MB
//
//	# Connection timeout
//	GRPC_MAX_CONNECTION_TIMEOUT=30s
//
//	# Rate limiting
//	GRPC_MAX_REQUESTS_PER_SECOND=1000
//	GRPC_MAX_REQUEST_BURST_CAPACITY=2000
//
//	# TLS
//	GRPC_TLS_ENABLED=true
//	GRPC_TLS_CERT_FILE=/etc/certs/server.crt
//	GRPC_TLS_KEY_FILE=/etc/certs/server.key
//	GRPC_TLS_MIN_VERSION=1.2
//
//	# Token auth
//	GRPC_ENABLE_TOKEN_AUTH=true
//	GRPC_TOKEN_SECRET=your-secret-key-min-32-chars
//
// This configuration:
//
//   - Listens on standard OTLP port 4317
//   - Accepts messages up to 16MB
//   - Limits to 1000 RPS with 2x burst
//   - Uses TLS 1.2+ for encryption
//   - Requires token authentication
//   - Times out slow connections after 30s
package grpc
