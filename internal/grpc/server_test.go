package grpc

import (
	"context"
	"net"
	"testing"

	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc/peer"
)

func TestServiceCreation(t *testing.T) {
	// Test creating individual services
	resilienceComponents := &interfaces.ResilienceComponents{}
	traceService := NewTraceService(nil, nil, resilienceComponents)
	metricsService := NewMetricsService(nil, nil)
	logsService := NewLogsService(nil, nil)

	assert.NotNil(t, traceService)
	assert.NotNil(t, metricsService)
	assert.NotNil(t, logsService)
}

func TestGRPCServiceIntegration(t *testing.T) {
	// Test that services can be created and configured properly
	resilienceComponents := &interfaces.ResilienceComponents{}

	// Create services
	traceService := NewTraceService(nil, nil, resilienceComponents)
	metricsService := NewMetricsService(nil, nil)
	logsService := NewLogsService(nil, nil)

	// Test they implement the correct interfaces
	assert.Implements(t, (*tracecollectorpb.TraceServiceServer)(nil), traceService)
	assert.Implements(t, (*metricscollectorpb.MetricsServiceServer)(nil), metricsService)
	assert.Implements(t, (*logscollectorpb.LogsServiceServer)(nil), logsService)
}

func TestServiceConfiguration(t *testing.T) {
	// Test service configuration with different handlers
	kinesisHandler := &streaming.KinesisHandler{}
	resilienceComponents := &interfaces.ResilienceComponents{}

	traceService := NewTraceService(kinesisHandler, nil, resilienceComponents)
	assert.NotNil(t, traceService)

	// Test that services can handle nil handlers gracefully
	traceServiceNil := NewTraceService(nil, nil, resilienceComponents)
	assert.NotNil(t, traceServiceNil)
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func() context.Context
		expectedIP     string
	}{
		{
			name: "IPv4 TCP address",
			setupContext: func() context.Context {
				tcpAddr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:12345")
				p := &peer.Peer{
					Addr: tcpAddr,
				}
				return peer.NewContext(context.Background(), p)
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "IPv6 loopback address",
			setupContext: func() context.Context {
				tcpAddr, _ := net.ResolveTCPAddr("tcp6", "[::1]:12345")
				p := &peer.Peer{
					Addr: tcpAddr,
				}
				return peer.NewContext(context.Background(), p)
			},
			expectedIP: "127.0.0.1",
		},
		{
			name: "IPv6-mapped IPv4 address",
			setupContext: func() context.Context {
				// Create IPv6-mapped IPv4 address manually
				ip := net.ParseIP("::ffff:192.168.1.100")
				tcpAddr := &net.TCPAddr{
					IP:   ip,
					Port: 12345,
				}
				p := &peer.Peer{
					Addr: tcpAddr,
				}
				return peer.NewContext(context.Background(), p)
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "context without peer info",
			setupContext: func() context.Context {
				return context.Background()
			},
			expectedIP: "127.0.0.1", // Default fallback
		},
		{
			name: "context with non-TCP address",
			setupContext: func() context.Context {
				// Create a mock address that's not TCP
				addr := &mockAddr{network: "unix", address: "/tmp/socket"}
				p := &peer.Peer{
					Addr: addr,
				}
				return peer.NewContext(context.Background(), p)
			},
			expectedIP: "127.0.0.1", // Should fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			ip := ExtractClientIP(ctx)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

// Mock address implementation for testing
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string {
	return m.network
}

func (m *mockAddr) String() string {
	return m.address
}

func TestResilienceComponents(t *testing.T) {
	// Test resilience components structure
	resilienceComponents := &interfaces.ResilienceComponents{}
	assert.NotNil(t, resilienceComponents)

	// Test that services can be created with resilience components
	traceService := NewTraceService(nil, nil, resilienceComponents)
	assert.NotNil(t, traceService)
}

func TestServiceExportMethods(t *testing.T) {
	// Test that export methods exist and can be called
	resilienceComponents := &interfaces.ResilienceComponents{}
	traceService := NewTraceService(nil, nil, resilienceComponents)

	// Test export method exists (will be tested in more detail in integration tests)
	ctx := context.Background()
	req := &tracecollectorpb.ExportTraceServiceRequest{}

	// This should not panic
	assert.NotPanics(t, func() {
		_, _ = traceService.Export(ctx, req)
	})
}

func TestServiceDependencies(t *testing.T) {
	// Test different service dependency configurations
	tests := []struct {
		name string
		kinesisHandler *streaming.KinesisHandler
		clickhouseHandler streaming.Handler
	}{
		{
			name: "nil handlers",
			kinesisHandler: nil,
			clickhouseHandler: nil,
		},
		{
			name: "kinesis handler only",
			kinesisHandler: &streaming.KinesisHandler{},
			clickhouseHandler: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resilienceComponents := &interfaces.ResilienceComponents{}
			traceService := NewTraceService(tt.kinesisHandler, tt.clickhouseHandler, resilienceComponents)
			assert.NotNil(t, traceService)
		})
	}
}

// Benchmark test for service creation
func BenchmarkServiceCreation(b *testing.B) {
	streamHandler := &streaming.KinesisHandler{}
	resilienceComponents := &interfaces.ResilienceComponents{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service := NewTraceService(streamHandler, nil, resilienceComponents)
		_ = service
	}
}

// Benchmark test for client IP extraction  
func BenchmarkExtractClientIP(b *testing.B) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:12345")
	p := &peer.Peer{
		Addr: tcpAddr,
	}
	ctx := peer.NewContext(context.Background(), p)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := ExtractClientIP(ctx)
		_ = ip
	}
}