package grpcapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RateLimitConfig defines the interface for rate limiting configuration
type RateLimitConfig interface {
	GetRequestsPerSecond() int
	GetBurstCapacity() int
}

// GRPCRateLimitingInterceptor creates a rate limiting interceptor for gRPC services
func GRPCRateLimitingInterceptor(cfg interface{}) grpc.UnaryServerInterceptor {
	// Default values
	requestsPerSecond := 100
	burstCapacity := 200

	// Try to extract configuration using reflection-like approach
	if rateCfg, ok := cfg.(RateLimitConfig); ok {
		requestsPerSecond = rateCfg.GetRequestsPerSecond()
		burstCapacity = rateCfg.GetBurstCapacity()
	}

	// Create global rate limiter for gRPC
	globalRateLimit := rate.Every(time.Second / time.Duration(requestsPerSecond))
	grpcLimiter := resilience.NewTenantRateLimiter(globalRateLimit, burstCapacity)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract client identifier
		clientID := extractGRPCClientIdentifier(ctx)

		// Apply rate limiting
		if !grpcLimiter.Allow(clientID) {
			logger.Warn("gRPC rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("method", info.FullMethod))

			return nil, status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded: too many requests from client %s", clientID)
		}

		// Continue with the request
		return handler(ctx, req)
	}
}

// MethodSpecificRateLimitingInterceptor applies different rate limits per gRPC method
func MethodSpecificRateLimitingInterceptor() grpc.UnaryServerInterceptor {
	// Different rate limiters for different methods
	traceLimiter := resilience.NewTenantRateLimiter(rate.Every(20*time.Millisecond), 100) // 50 RPS, high volume
	metricsLimiter := resilience.NewTenantRateLimiter(rate.Every(50*time.Millisecond), 60) // 20 RPS, medium volume
	logsLimiter := resilience.NewTenantRateLimiter(rate.Every(30*time.Millisecond), 80)    // 33 RPS, high volume

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		clientID := extractGRPCClientIdentifier(ctx)

		// Select appropriate limiter based on method
		var limiter *resilience.TenantRateLimiter
		var methodType string

		switch {
		case strings.Contains(info.FullMethod, "TraceService"):
			limiter = traceLimiter
			methodType = "trace"
		case strings.Contains(info.FullMethod, "MetricsService"):
			limiter = metricsLimiter
			methodType = "metrics"
		case strings.Contains(info.FullMethod, "LogsService"):
			limiter = logsLimiter
			methodType = "logs"
		default:
			// Default conservative rate limiting for unknown methods
			limiter = metricsLimiter
			methodType = "default"
		}

		if !limiter.Allow(clientID) {
			logger.Warn("gRPC method-specific rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("method", info.FullMethod),
				zap.String("method_type", methodType))

			return nil, status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded for %s methods: too many requests from client %s",
				methodType, clientID)
		}

		return handler(ctx, req)
	}
}

// SizeBasedGRPCRateLimitingInterceptor applies rate limits based on request size
func SizeBasedGRPCRateLimitingInterceptor() grpc.UnaryServerInterceptor {
	smallRequestLimiter := resilience.NewTenantRateLimiter(rate.Every(50*time.Millisecond), 40)  // 20 RPS for small
	largeRequestLimiter := resilience.NewTenantRateLimiter(rate.Every(200*time.Millisecond), 10) // 5 RPS for large

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		clientID := extractGRPCClientIdentifier(ctx)

		// Estimate request size (this is approximate)
		requestSize := estimateGRPCRequestSize(req)

		var limiter *resilience.TenantRateLimiter
		var sizeCategory string

		if requestSize > 1024*1024 { // > 1MB
			limiter = largeRequestLimiter
			sizeCategory = "large"
		} else {
			limiter = smallRequestLimiter
			sizeCategory = "small"
		}

		if !limiter.Allow(clientID) {
			logger.Warn("gRPC size-based rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("method", info.FullMethod),
				zap.String("size_category", sizeCategory),
				zap.Int64("estimated_size", requestSize))

			return nil, status.Errorf(codes.ResourceExhausted,
				"rate limit exceeded for %s requests: request size %d bytes from client %s",
				sizeCategory, requestSize, clientID)
		}

		return handler(ctx, req)
	}
}

// extractGRPCClientIdentifier extracts a unique client identifier from gRPC context
func extractGRPCClientIdentifier(ctx context.Context) string {
	// Try to get tenant ID from metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if tenantIDs := md.Get("x-tenant-id"); len(tenantIDs) > 0 {
			return "tenant:" + tenantIDs[0]
		}
		if apiKeys := md.Get("x-api-key"); len(apiKeys) > 0 {
			return "api_key:" + apiKeys[0]
		}
	}

	// Fall back to client IP
	if peer, ok := peer.FromContext(ctx); ok {
		if tcpAddr, ok := peer.Addr.(*net.TCPAddr); ok {
			return "ip:" + tcpAddr.IP.String()
		}
		return "addr:" + peer.Addr.String()
	}

	return "unknown"
}

// estimateGRPCRequestSize calculates the actual protobuf size of the request
func estimateGRPCRequestSize(req interface{}) int64 {
	if protoMsg, ok := req.(interface{ ProtoSize() int }); ok {
		return int64(protoMsg.ProtoSize())
	}

	// Fallback to proto.Size for standard protobuf messages
	if protoMsg, ok := req.(interface{ Size() int }); ok {
		return int64(protoMsg.Size())
	}

	// Final fallback using string length as approximation
	return int64(len(fmt.Sprintf("%+v", req)))
}

// ChainGRPCInterceptors combines multiple gRPC interceptors
func ChainGRPCInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Build chain of interceptors
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chained
			chained = func(ctx context.Context, req interface{}) (interface{}, error) {
				return interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
					return next(ctx, req)
				})
			}
		}
		return chained(ctx, req)
	}
}