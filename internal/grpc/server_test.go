package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/peer"
)

func TestNewServer(t *testing.T) {
	server := NewServer("localhost:0", nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.grpcServer)
	assert.Equal(t, "localhost:0", server.addr)
}

func TestServer_StartStop(t *testing.T) {
	// Use port 0 to get an available port
	server := NewServer("localhost:0", nil, nil)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		err := server.Start()
		errCh <- err
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	server.Stop()

	// Check if server stopped gracefully
	select {
	case err := <-errCh:
		// Server should stop without error when gracefully stopped
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestServer_ServiceRegistration(t *testing.T) {
	server := NewServer("localhost:0", nil, nil)

	// Test that the gRPC server has the expected services registered
	serviceInfo := server.grpcServer.GetServiceInfo()

	// Check that all OTLP services are registered
	expectedServices := []string{
		"opentelemetry.proto.collector.trace.v1.TraceService",
		"opentelemetry.proto.collector.metrics.v1.MetricsService", 
		"opentelemetry.proto.collector.logs.v1.LogsService",
		"grpc.reflection.v1alpha.ServerReflection", // Reflection service
	}

	assert.GreaterOrEqual(t, len(serviceInfo), 3, "Should have at least 3 OTLP services registered")

	for _, expectedService := range expectedServices[:3] { // Skip reflection service as it might have different name
		found := false
		for serviceName := range serviceInfo {
			if serviceName == expectedService {
				found = true
				break
			}
		}
		assert.True(t, found, "Service %s should be registered", expectedService)
	}
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

func TestServer_Integration(t *testing.T) {
	// Create a server with nil handlers for testing
	server := NewServer("localhost:0", nil, nil)
	
	// Test that server can be created and configured
	assert.NotNil(t, server)
	assert.NotNil(t, server.grpcServer)
	
	// Test graceful stop without start (should not panic)
	assert.NotPanics(t, func() {
		server.Stop()
	})
}

func TestServer_ServiceMethods(t *testing.T) {
	server := NewServer("localhost:0", nil, nil)

	// Get service info to check that methods are properly registered
	serviceInfo := server.grpcServer.GetServiceInfo()

	// Check trace service methods
	if traceServiceInfo, ok := serviceInfo["opentelemetry.proto.collector.trace.v1.TraceService"]; ok {
		hasExport := false
		for _, method := range traceServiceInfo.Methods {
			if method.Name == "Export" {
				hasExport = true
				break
			}
		}
		assert.True(t, hasExport, "TraceService should have Export method")
	}

	// Check metrics service methods
	if metricsServiceInfo, ok := serviceInfo["opentelemetry.proto.collector.metrics.v1.MetricsService"]; ok {
		hasExport := false
		for _, method := range metricsServiceInfo.Methods {
			if method.Name == "Export" {
				hasExport = true
				break
			}
		}
		assert.True(t, hasExport, "MetricsService should have Export method")
	}

	// Check logs service methods  
	if logsServiceInfo, ok := serviceInfo["opentelemetry.proto.collector.logs.v1.LogsService"]; ok {
		hasExport := false
		for _, method := range logsServiceInfo.Methods {
			if method.Name == "Export" {
				hasExport = true
				break
			}
		}
		assert.True(t, hasExport, "LogsService should have Export method")
	}
}

func TestServer_Configuration(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "localhost with port",
			addr: "localhost:4317",
		},
		{
			name: "IP address with port",
			addr: "127.0.0.1:4317",
		},
		{
			name: "any interface",
			addr: ":4317",
		},
		{
			name: "dynamic port",
			addr: "localhost:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.addr, nil, nil)
			assert.NotNil(t, server)
			assert.Equal(t, tt.addr, server.addr)
		})
	}
}

// Benchmark test for server creation
func BenchmarkNewServer(b *testing.B) {
	streamHandler := &streaming.KinesisHandler{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := NewServer("localhost:0", streamHandler, nil)
		_ = server
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