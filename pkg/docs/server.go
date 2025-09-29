package docs

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/errors"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server provides documentation serving capabilities
type Server struct {
	generator *Generator
	config    *config.ConsolidatedConfig
	templates *template.Template
}

// NewServer creates a new documentation server
func NewServer(cfg *config.ConsolidatedConfig) (*Server, error) {
	// Load templates
	templates, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, errors.InternalServer("Failed to load documentation templates", err)
	}

	return &Server{
		generator: NewGenerator(cfg),
		config:    cfg,
		templates: templates,
	}, nil
}

// SetupRoutes configures documentation routes
func (s *Server) SetupRoutes(router gin.IRouter) {
	docs := router.Group("/docs")
	{
		// Interactive API documentation
		docs.GET("/", s.serveDocs)
		docs.GET("/api", s.serveDocs)

		// OpenAPI specification endpoints
		docs.GET("/openapi.json", s.serveOpenAPIJSON)
		docs.GET("/openapi.yaml", s.serveOpenAPIYAML)

		// Static assets (CSS, JS, images)
		docs.GET("/static/*filepath", s.serveStatic)

		// API reference pages
		docs.GET("/reference/:section", s.serveReference)

		// Examples and guides
		docs.GET("/examples", s.serveExamples)
		docs.GET("/examples/:category", s.serveExampleCategory)

		// Authentication guide
		docs.GET("/auth", s.serveAuthGuide)

		// Rate limiting documentation
		docs.GET("/rate-limits", s.serveRateLimits)

		// Error codes reference
		docs.GET("/errors", s.serveErrorCodes)

		// SDK documentation
		docs.GET("/sdks", s.serveSDKs)
		docs.GET("/sdks/:language", s.serveSDK)
	}
}

// serveDocs serves the main interactive documentation page
func (s *Server) serveDocs(c *gin.Context) {
	data := map[string]interface{}{
		"Title":       "Playback API Documentation",
		"Version":     s.config.App.Version,
		"Environment": s.config.App.Environment,
		"BaseURL":     s.getBaseURL(),
		"Features": []map[string]interface{}{
			{
				"Name":        "High-Performance Ingestion",
				"Description": "Process millions of telemetry events per second with sub-millisecond latency",
				"Icon":        "⚡",
			},
			{
				"Name":        "OpenTelemetry Native",
				"Description": "Full OTLP support for traces, metrics, and logs",
				"Icon":        "📊",
			},
			{
				"Name":        "Real-time Queries",
				"Description": "Query your telemetry data in real-time with ClickHouse",
				"Icon":        "🔍",
			},
			{
				"Name":        "Distributed Tracing",
				"Description": "W3C-compliant trace correlation across services",
				"Icon":        "🔗",
			},
		},
		"QuickStart": s.generateQuickStartSteps(),
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "docs.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render documentation", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveOpenAPIJSON serves the OpenAPI specification as JSON
func (s *Server) serveOpenAPIJSON(c *gin.Context) {
	jsonData, err := s.generator.ToJSON()
	if err != nil {
		errors.AbortInternalServer(c, "Failed to generate OpenAPI specification", err)
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	c.Data(http.StatusOK, "application/json", jsonData)
}

// serveOpenAPIYAML serves the OpenAPI specification as YAML
func (s *Server) serveOpenAPIYAML(c *gin.Context) {
	spec, err := s.generator.GenerateSpec()
	if err != nil {
		errors.AbortInternalServer(c, "Failed to generate OpenAPI specification", err)
		return
	}

	yamlData, err := s.convertToYAML(spec)
	if err != nil {
		errors.AbortInternalServer(c, "Failed to convert specification to YAML", err)
		return
	}

	c.Header("Content-Type", "application/yaml")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/yaml", yamlData)
}

// serveStatic serves static assets
func (s *Server) serveStatic(c *gin.Context) {
	filepath := c.Param("filepath")

	// Security: prevent directory traversal
	if strings.Contains(filepath, "..") {
		errors.AbortBadRequest(c, "Invalid file path")
		return
	}

	// Remove leading slash
	filepath = strings.TrimPrefix(filepath, "/")

	data, err := staticFS.ReadFile("static/" + filepath)
	if err != nil {
		errors.AbortNotFound(c, "static file")
		return
	}

	// Set appropriate content type
	contentType := s.getContentType(filepath)
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	c.Data(http.StatusOK, contentType, data)
}

// serveReference serves API reference documentation
func (s *Server) serveReference(c *gin.Context) {
	section := c.Param("section")

	var templateName string
	var data map[string]interface{}

	switch section {
	case "traces":
		templateName = "traces.html"
		data = s.getTracesReferenceData()
	case "metrics":
		templateName = "metrics.html"
		data = s.getMetricsReferenceData()
	case "logs":
		templateName = "logs.html"
		data = s.getLogsReferenceData()
	case "replay":
		templateName = "replay.html"
		data = s.getReplayReferenceData()
	default:
		errors.AbortNotFound(c, "documentation section")
		return
	}

	if err := s.templates.ExecuteTemplate(c.Writer, templateName, data); err != nil {
		errors.AbortInternalServer(c, "Failed to render reference documentation", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveExamples serves code examples
func (s *Server) serveExamples(c *gin.Context) {
	data := map[string]interface{}{
		"Title": "API Examples",
		"Categories": []map[string]interface{}{
			{
				"Name":        "traces",
				"Title":       "Trace Examples",
				"Description": "Examples for ingesting and querying trace data",
				"Icon":        "🔍",
			},
			{
				"Name":        "metrics",
				"Title":       "Metrics Examples",
				"Description": "Examples for metrics ingestion and aggregation",
				"Icon":        "📊",
			},
			{
				"Name":        "logs",
				"Title":       "Logs Examples",
				"Description": "Examples for structured log ingestion",
				"Icon":        "📝",
			},
			{
				"Name":        "replay",
				"Title":       "Session Replay",
				"Description": "Examples for session replay data",
				"Icon":        "🎬",
			},
		},
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "examples.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render examples", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveExampleCategory serves examples for a specific category
func (s *Server) serveExampleCategory(c *gin.Context) {
	category := c.Param("category")

	var examples []map[string]interface{}

	switch category {
	case "traces":
		examples = s.getTraceExamples()
	case "metrics":
		examples = s.getMetricsExamples()
	case "logs":
		examples = s.getLogsExamples()
	case "replay":
		examples = s.getReplayExamples()
	default:
		errors.AbortNotFound(c, "example category")
		return
	}

	data := map[string]interface{}{
		"Title":    strings.ToUpper(category[:1]) + strings.ToLower(category[1:]) + " Examples",
		"Category": category,
		"Examples": examples,
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "example-category.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render example category", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveAuthGuide serves authentication documentation
func (s *Server) serveAuthGuide(c *gin.Context) {
	data := map[string]interface{}{
		"Title": "Authentication Guide",
		"AuthMethods": []map[string]interface{}{
			{
				"Name":        "JWT Bearer Token",
				"Type":        "bearer",
				"Description": "Recommended for production applications",
				"Header":      "Authorization: Bearer <token>",
				"Example":     "Authorization: Bearer eyJhbGciOiJIUzI1NiIs...",
			},
			{
				"Name":        "API Key",
				"Type":        "apikey",
				"Description": "Simple authentication for development",
				"Header":      "X-API-Key: <key>",
				"Example":     "X-API-Key: pk_live_1234567890abcdef",
			},
		},
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "auth.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render auth guide", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveRateLimits serves rate limiting documentation
func (s *Server) serveRateLimits(c *gin.Context) {
	data := map[string]interface{}{
		"Title": "Rate Limits",
		"Limits": []map[string]interface{}{
			{
				"Endpoint":    "/api/v1/traces",
				"Method":      "POST",
				"Limit":       "50 requests/second",
				"Burst":       "100 requests",
				"PayloadSize": "25 MB",
			},
			{
				"Endpoint":    "/api/v1/metrics",
				"Method":      "POST",
				"Limit":       "30 requests/second",
				"Burst":       "60 requests",
				"PayloadSize": "10 MB",
			},
			{
				"Endpoint":    "/api/v1/logs",
				"Method":      "POST",
				"Limit":       "40 requests/second",
				"Burst":       "80 requests",
				"PayloadSize": "15 MB",
			},
		},
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "rate-limits.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render rate limits", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveErrorCodes serves error codes reference
func (s *Server) serveErrorCodes(c *gin.Context) {
	data := map[string]interface{}{
		"Title": "Error Codes Reference",
		"ErrorCodes": s.generateErrorCodesData(),
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "error-codes.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render error codes", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveSDKs serves SDK documentation overview
func (s *Server) serveSDKs(c *gin.Context) {
	data := map[string]interface{}{
		"Title": "Official SDKs",
		"SDKs": []map[string]interface{}{
			{
				"Language":    "go",
				"Name":        "Go SDK",
				"Description": "High-performance Go client for Playback API",
				"Status":      "stable",
				"Repository":  "https://github.com/playback/playback-go",
			},
			{
				"Language":    "javascript",
				"Name":        "Node.js SDK",
				"Description": "TypeScript/JavaScript client for Node.js applications",
				"Status":      "stable",
				"Repository":  "https://github.com/playback/playback-js",
			},
			{
				"Language":    "python",
				"Name":        "Python SDK",
				"Description": "Python client with asyncio support",
				"Status":      "beta",
				"Repository":  "https://github.com/playback/playback-python",
			},
			{
				"Language":    "java",
				"Name":        "Java SDK",
				"Description": "Java client for enterprise applications",
				"Status":      "alpha",
				"Repository":  "https://github.com/playback/playback-java",
			},
		},
	}

	if err := s.templates.ExecuteTemplate(c.Writer, "sdks.html", data); err != nil {
		errors.AbortInternalServer(c, "Failed to render SDKs", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// serveSDK serves language-specific SDK documentation
func (s *Server) serveSDK(c *gin.Context) {
	language := c.Param("language")

	var data map[string]interface{}
	var templateName string

	switch language {
	case "go":
		data = s.getGoSDKData()
		templateName = "sdk-go.html"
	case "javascript", "js", "node":
		data = s.getJavaScriptSDKData()
		templateName = "sdk-js.html"
	case "python":
		data = s.getPythonSDKData()
		templateName = "sdk-python.html"
	case "java":
		data = s.getJavaSDKData()
		templateName = "sdk-java.html"
	default:
		errors.AbortNotFound(c, "SDK language")
		return
	}

	if err := s.templates.ExecuteTemplate(c.Writer, templateName, data); err != nil {
		errors.AbortInternalServer(c, "Failed to render SDK documentation", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
}

// Helper methods

func (s *Server) getBaseURL() string {
	if s.config.App.Environment == "production" {
		return "https://api.playback.com"
	}
	return fmt.Sprintf("http://%s:%d", s.config.Network.HTTP.Host, s.config.Network.HTTP.Port)
}

func (s *Server) getContentType(filepath string) string {
	switch {
	case strings.HasSuffix(filepath, ".css"):
		return "text/css"
	case strings.HasSuffix(filepath, ".js"):
		return "application/javascript"
	case strings.HasSuffix(filepath, ".png"):
		return "image/png"
	case strings.HasSuffix(filepath, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(filepath, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) generateQuickStartSteps() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"Step":        1,
			"Title":       "Get API Key",
			"Description": "Sign up and get your API key from the dashboard",
			"Code":        `export PLAYBACK_API_KEY="pk_live_..."`,
		},
		{
			"Step":        2,
			"Title":       "Send Your First Trace",
			"Description": "Use curl to send OpenTelemetry trace data",
			"Code":        s.generateCurlExample("traces"),
		},
		{
			"Step":        3,
			"Title":       "Query Your Data",
			"Description": "Retrieve and analyze your telemetry data",
			"Code":        s.generateQueryExample(),
		},
		{
			"Step":        4,
			"Title":       "Integrate SDKs",
			"Description": "Use our official SDKs for production integration",
			"Code":        s.generateSDKExample(),
		},
	}
}

func (s *Server) generateCurlExample(dataType string) string {
	baseURL := s.getBaseURL()

	examples := map[string]string{
		"traces": fmt.Sprintf(`curl -X POST %s/api/v1/traces \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $PLAYBACK_API_KEY" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "my-service"}
        }]
      }
    }]
  }'`, baseURL),
	}

	return examples[dataType]
}

func (s *Server) generateQueryExample() string {
	return fmt.Sprintf(`curl -X GET %s/api/v1/traces/1234567890abcdef \
  -H "X-API-Key: $PLAYBACK_API_KEY"`, s.getBaseURL())
}

func (s *Server) generateSDKExample() string {
	return `go get github.com/playback/playback-go

import "github.com/playback/playback-go"

client := playback.NewClient("pk_live_...")
err := client.SendTrace(ctx, traceData)`
}

func (s *Server) convertToYAML(spec *OpenAPISpec) ([]byte, error) {
	// This would use a YAML library like gopkg.in/yaml.v3
	// For brevity, returning JSON for now
	return s.generator.ToJSON()
}

// Data generation methods for different sections
func (s *Server) getTracesReferenceData() map[string]interface{} {
	return map[string]interface{}{
		"Title":       "Traces API Reference",
		"Description": "OpenTelemetry trace ingestion and querying",
		"Endpoints": []map[string]interface{}{
			{
				"Method":      "POST",
				"Path":        "/api/v1/traces",
				"Summary":     "Ingest trace data",
				"Description": "Accept OTLP trace data for high-performance ingestion",
				"PayloadSize": "Up to 25 MB",
				"RateLimit":   "50 requests/second",
			},
			{
				"Method":      "GET",
				"Path":        "/api/v1/traces/{id}",
				"Summary":     "Get trace by ID",
				"Description": "Retrieve trace information from ClickHouse",
			},
		},
	}
}

func (s *Server) getMetricsReferenceData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Metrics API Reference",
		"Endpoints": []map[string]interface{}{
			{
				"Method":  "POST",
				"Path":    "/api/v1/metrics",
				"Summary": "Ingest metrics data",
			},
		},
	}
}

func (s *Server) getLogsReferenceData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Logs API Reference",
	}
}

func (s *Server) getReplayReferenceData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Session Replay API Reference",
	}
}

func (s *Server) getTraceExamples() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"Title":       "Basic Trace Ingestion",
			"Description": "Send a simple trace with one span",
			"Language":    "curl",
			"Code":        s.generateCurlExample("traces"),
		},
	}
}

func (s *Server) getMetricsExamples() []map[string]interface{} {
	return []map[string]interface{}{}
}

func (s *Server) getLogsExamples() []map[string]interface{} {
	return []map[string]interface{}{}
}

func (s *Server) getReplayExamples() []map[string]interface{} {
	return []map[string]interface{}{}
}

func (s *Server) generateErrorCodesData() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"Code":        "BAD_REQUEST",
			"HTTPStatus":  400,
			"Category":    "client",
			"Description": "Invalid request parameters or malformed request body",
			"Retryable":   false,
		},
		{
			"Code":        "VALIDATION_FAILED",
			"HTTPStatus":  400,
			"Category":    "client",
			"Description": "Request validation failed for one or more fields",
			"Retryable":   false,
		},
		{
			"Code":        "RATE_LIMITED",
			"HTTPStatus":  429,
			"Category":    "business",
			"Description": "Request rate limit exceeded for your API key",
			"Retryable":   true,
		},
		{
			"Code":        "SERVICE_UNAVAILABLE",
			"HTTPStatus":  503,
			"Category":    "server",
			"Description": "Service temporarily unavailable due to maintenance or high load",
			"Retryable":   true,
		},
	}
}

func (s *Server) getGoSDKData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Go SDK Documentation",
		"Installation": `go get github.com/playback/playback-go`,
		"QuickStart": `package main

import (
	"context"
	"github.com/playback/playback-go"
)

func main() {
	client := playback.NewClient("pk_live_...")

	err := client.SendTrace(context.Background(), traceData)
	if err != nil {
		panic(err)
	}
}`,
	}
}

func (s *Server) getJavaScriptSDKData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "JavaScript SDK Documentation",
		"Installation": `npm install @playback/playback-js`,
	}
}

func (s *Server) getPythonSDKData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Python SDK Documentation",
		"Installation": `pip install playback-python`,
	}
}

func (s *Server) getJavaSDKData() map[string]interface{} {
	return map[string]interface{}{
		"Title": "Java SDK Documentation",
		"Installation": `<!-- Maven -->
<dependency>
	<groupId>com.playback</groupId>
	<artifactId>playback-java</artifactId>
	<version>1.0.0</version>
</dependency>`,
	}
}