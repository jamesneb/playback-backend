package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSize_Constants(t *testing.T) {
	tests := []struct {
		name     string
		size     Size
		expected int64
	}{
		{"1 KB", KB, 1024},
		{"1 MB", MB, 1048576},
		{"Default max request", DefaultMaxRequestSize, 10 * 1048576},
		{"Max allowed request", MaxAllowedRequestSize, 50 * 1048576},
		{"Trace max request", TraceMaxRequestSize, 25 * 1048576},
		{"Metrics max request", MetricsMaxRequestSize, 10 * 1048576},
		{"Logs max request", LogsMaxRequestSize, 15 * 1048576},
		{"Replay max request", ReplayMaxRequestSize, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int64(tt.size))
		})
	}
}

func TestPayloadSizeLimit_WithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	middleware := PayloadSizeLimit(1 * KB) // 1KB limit

	// Create test payload under the limit
	testPayload := strings.Repeat("a", 512) // 512 bytes

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute middleware
	called := false
	finalHandler := func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	}

	middleware(c)
	if !c.IsAborted() {
		finalHandler(c)
	}

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPayloadSizeLimit_ExceedsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	middleware := PayloadSizeLimit(1 * KB) // 1KB limit

	// Create test payload exceeding the limit
	testPayload := strings.Repeat("a", 2048) // 2KB payload

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute middleware
	called := false
	finalHandler := func(c *gin.Context) {
		called = true
	}

	middleware(c)
	if !c.IsAborted() {
		finalHandler(c)
	}

	assert.False(t, called)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	// Check error response
	assert.Contains(t, w.Body.String(), "Request body too large")
	assert.Contains(t, w.Body.String(), "maxSize")
}

func TestPayloadSizeLimit_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	middleware := PayloadSizeLimit(1 * KB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// Execute middleware
	called := false
	finalHandler := func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	}

	middleware(c)
	if !c.IsAborted() {
		finalHandler(c)
	}

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPayloadSizeLimit_BoundaryCondition(t *testing.T) {
	gin.SetMode(gin.TestMode)

	middleware := PayloadSizeLimit(1 * KB) // 1KB limit

	// Create test payload exactly at the limit
	testPayload := strings.Repeat("a", 1024) // Exactly 1KB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute middleware
	called := false
	finalHandler := func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	}

	middleware(c)
	if !c.IsAborted() {
		finalHandler(c)
	}

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContentLengthHandling(t *testing.T) {
	tests := []struct {
		name           string
		contentLength  string
		payload        string
		limit          Size
		shouldPass     bool
	}{
		{"valid content length under limit", "1024", strings.Repeat("a", 1024), 2*KB, true},
		{"valid content length over limit", "2048", strings.Repeat("a", 2048), 1*KB, false},
		{"zero content length", "0", "", 1*KB, true},
		{"missing content length header", "", "test", 1*KB, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := PayloadSizeLimit(tt.limit)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.payload))
			if tt.contentLength != "" {
				req.Header.Set("Content-Length", tt.contentLength)
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			called := false
			finalHandler := func(c *gin.Context) {
				called = true
				c.Status(http.StatusOK)
			}

			middleware(c)
			if !c.IsAborted() {
				finalHandler(c)
			}

			if tt.shouldPass {
				assert.True(t, called)
			} else {
				assert.False(t, called)
				assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			}
		})
	}
}

func TestPayloadSizeLimitErrorResponse(t *testing.T) {
	middleware := PayloadSizeLimit(1 * KB)
	testPayload := strings.Repeat("a", 2048) // 2KB

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	middleware(c)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "Request body too large")
	assert.Contains(t, w.Body.String(), "maxSize")
}

func TestPayloadSizeLimit_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Apply different size limits to different routes
	router.POST("/traces", PayloadSizeLimit(TraceMaxRequestSize), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/metrics", PayloadSizeLimit(MetricsMaxRequestSize), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/logs", PayloadSizeLimit(LogsMaxRequestSize), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test trace endpoint with large payload (should pass)
	largeTracePayload := strings.Repeat("a", 20*1024*1024) // 20MB
	req := httptest.NewRequest("POST", "/traces", strings.NewReader(largeTracePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test metrics endpoint with large payload (should fail)
	largeMetricsPayload := strings.Repeat("a", 15*1024*1024) // 15MB
	req = httptest.NewRequest("POST", "/metrics", strings.NewReader(largeMetricsPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestSpecificPayloadLimitFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		middlewareFunc  func() gin.HandlerFunc
		expectedLimit   Size
		payloadSize     int
		shouldPass      bool
	}{
		{"TracePayloadLimit under limit", TracePayloadLimit, TraceMaxRequestSize, 20*1024*1024, true},
		{"TracePayloadLimit over limit", TracePayloadLimit, TraceMaxRequestSize, 30*1024*1024, false},
		{"MetricsPayloadLimit under limit", MetricsPayloadLimit, MetricsMaxRequestSize, 8*1024*1024, true},
		{"MetricsPayloadLimit over limit", MetricsPayloadLimit, MetricsMaxRequestSize, 12*1024*1024, false},
		{"LogsPayloadLimit under limit", LogsPayloadLimit, LogsMaxRequestSize, 10*1024*1024, true},
		{"LogsPayloadLimit over limit", LogsPayloadLimit, LogsMaxRequestSize, 18*1024*1024, false},
		{"ReplayPayloadLimit under limit", ReplayPayloadLimit, ReplayMaxRequestSize, 512, true},
		{"ReplayPayloadLimit over limit", ReplayPayloadLimit, ReplayMaxRequestSize, 2048, false},
		{"DefaultPayloadLimit under limit", DefaultPayloadLimit, DefaultMaxRequestSize, 8*1024*1024, true},
		{"DefaultPayloadLimit over limit", DefaultPayloadLimit, DefaultMaxRequestSize, 12*1024*1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := tt.middlewareFunc()

			testPayload := strings.Repeat("a", tt.payloadSize)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
			c.Request.Header.Set("Content-Type", "application/json")

			called := false
			finalHandler := func(c *gin.Context) {
				called = true
				c.Status(http.StatusOK)
			}

			middleware(c)
			if !c.IsAborted() {
				finalHandler(c)
			}

			if tt.shouldPass {
				assert.True(t, called, "Handler should have been called")
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.False(t, called, "Handler should not have been called")
				assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			}
		})
	}
}

func TestPayloadSizeLimit_StreamingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware := PayloadSizeLimit(1 * KB)

	// Simulate streaming request without Content-Length header
	testPayload := strings.Repeat("a", 2048) // 2KB
	req := httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
	req.Header.Del("Content-Length") // Remove content-length to simulate streaming
	req.Header.Set("Transfer-Encoding", "chunked")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	called := false
	finalHandler := func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	}

	middleware(c)
	if !c.IsAborted() {
		finalHandler(c)
	}

	// The middleware still checks Content-Length header even for chunked requests
	// In this case, Content-Length was set by the test request creation
	// so it will be blocked by the size limit
	assert.False(t, called)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func BenchmarkPayloadSizeLimit(b *testing.B) {
	gin.SetMode(gin.TestMode)
	middleware := PayloadSizeLimit(1 * MB)

	testPayload := strings.Repeat("a", 1024) // 1KB payload

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(testPayload))
		c.Request.Header.Set("Content-Type", "application/json")

		middleware(c)
	}
}

func BenchmarkSize_Conversion(b *testing.B) {
	sizes := []Size{
		Size(500),
		Size(1024),
		Size(1048576),
		Size(2621440),
		Size(52428800),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		_ = int64(size)
	}
}