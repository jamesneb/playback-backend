package grpc

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc/peer"
)

// ExtractClientIP extracts the client IP address from gRPC context
func ExtractClientIP(ctx context.Context) string {
	// Extract peer information from gRPC context
	if p, ok := peer.FromContext(ctx); ok {
		if tcpAddr, ok := p.Addr.(*net.TCPAddr); ok && tcpAddr.IP != nil {
			ip := tcpAddr.IP.String()
			// Handle IPv6 loopback and convert to IPv4
			if ip == "::1" {
				return "127.0.0.1"
			}
			// Extract IPv4 from IPv6-mapped addresses
			if strings.HasPrefix(ip, "::ffff:") {
				return strings.TrimPrefix(ip, "::ffff:")
			}
			return ip
		}
		// Fallback: parse address string
		addr := p.Addr.String()
		if host, _, err := net.SplitHostPort(addr); err == nil {
			if net.ParseIP(host) != nil {
				return host
			}
		}
	}
	// Default fallback for local connections
	return "127.0.0.1"
}
