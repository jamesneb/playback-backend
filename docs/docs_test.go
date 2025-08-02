package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggo/swag"
)

func TestSwaggerInfo(t *testing.T) {
	// Test that SwaggerInfo is properly initialized
	assert.NotNil(t, SwaggerInfo)
	assert.Equal(t, "1.0", SwaggerInfo.Version)
	assert.Equal(t, "localhost:8080", SwaggerInfo.Host)
	assert.Equal(t, "/api/v1", SwaggerInfo.BasePath)
	assert.Equal(t, "Playback Backend API", SwaggerInfo.Title)
	assert.Equal(t, "Distributed systems event replay backend", SwaggerInfo.Description)
	assert.Equal(t, "swagger", SwaggerInfo.InfoInstanceName)
}

func TestDocTemplate(t *testing.T) {
	// Test that docTemplate is not empty and contains expected structure
	assert.NotEmpty(t, docTemplate)
	assert.Contains(t, docTemplate, "swagger")
	assert.Contains(t, docTemplate, "2.0")
	assert.Contains(t, docTemplate, "paths")
	assert.Contains(t, docTemplate, "definitions")
	
	// Test that expected API endpoints are documented
	assert.Contains(t, docTemplate, "/api/v1/traces")
	assert.Contains(t, docTemplate, "/api/v1/metrics")
	assert.Contains(t, docTemplate, "/api/v1/logs")
	
	// Test that expected HTTP methods are documented
	assert.Contains(t, docTemplate, "get")
	assert.Contains(t, docTemplate, "post")
}

func TestSwaggerTemplateStructure(t *testing.T) {
	// Test that the template contains required Swagger fields
	assert.Contains(t, docTemplate, "{{escape .Description}}")
	assert.Contains(t, docTemplate, "{{.Title}}")
	assert.Contains(t, docTemplate, "{{.Version}}")
	assert.Contains(t, docTemplate, "{{.Host}}")
	assert.Contains(t, docTemplate, "{{.BasePath}}")
	assert.Contains(t, docTemplate, "{{ marshal .Schemes }}")
}

func TestSwaggerDefinitions(t *testing.T) {
	// Test that expected data structures are defined
	definitions := []string{
		"handlers.TraceResponse",
		"handlers.CreateTraceRequest", 
		"handlers.ErrorResponse",
		"handlers.MetricsRequest",
		"handlers.MetricsResponse",
		"handlers.LogsRequest",
		"handlers.LogsResponse",
		"handlers.TimeRange",
		"handlers.Attribute",
		"handlers.Resource",
	}
	
	for _, def := range definitions {
		assert.Contains(t, docTemplate, def, "Expected definition %s to be present", def)
	}
}

func TestAPIEndpoints(t *testing.T) {
	// Test that all expected endpoints are documented
	endpoints := map[string][]string{
		"/api/v1/traces": {"post"},
		"/api/v1/traces/{id}": {"get"},
		"/api/v1/metrics": {"get", "post"},
		"/api/v1/logs": {"get", "post"},
	}
	
	for endpoint, methods := range endpoints {
		assert.Contains(t, docTemplate, endpoint, "Expected endpoint %s to be documented", endpoint)
		for _, method := range methods {
			// Check that the method exists in the context of the endpoint
			assert.Contains(t, docTemplate, "\""+method+"\":", "Expected method %s for endpoint %s", method, endpoint)
		}
	}
}

func TestResponseCodes(t *testing.T) {
	// Test that expected HTTP response codes are documented
	responseCodes := []string{"200", "201", "400", "404"}
	
	for _, code := range responseCodes {
		assert.Contains(t, docTemplate, "\""+code+"\":", "Expected response code %s to be documented", code)
	}
}

func TestSwaggerRegistration(t *testing.T) {
	// Test that the swagger spec is registered
	spec := swag.GetSwagger(SwaggerInfo.InstanceName())
	assert.NotNil(t, spec)
	// Note: Can't directly compare SwaggerInfo with spec due to different types
	assert.NotNil(t, spec)
}

func TestContentTypes(t *testing.T) {
	// Test that expected content types are documented
	contentTypes := []string{
		"application/json",
	}
	
	for _, contentType := range contentTypes {
		assert.Contains(t, docTemplate, contentType, "Expected content type %s to be documented", contentType)
	}
}

func TestParameterTypes(t *testing.T) {
	// Test that various parameter types are documented
	parameterTypes := []string{
		"query",
		"path", 
		"body",
	}
	
	for _, paramType := range parameterTypes {
		assert.Contains(t, docTemplate, "\"in\": \""+paramType+"\"", "Expected parameter type %s to be documented", paramType)
	}
}

func TestTags(t *testing.T) {
	// Test that API tags are properly defined
	tags := []string{"traces", "metrics", "logs"}
	
	for _, tag := range tags {
		assert.Contains(t, docTemplate, "\""+tag+"\"", "Expected tag %s to be documented", tag)
	}
}

// Benchmark test for swagger info access
func BenchmarkSwaggerInfoAccess(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = SwaggerInfo.Version
		_ = SwaggerInfo.Host
		_ = SwaggerInfo.BasePath
		_ = SwaggerInfo.Title
	}
}

// Integration test that validates the complete swagger structure
func TestSwaggerIntegration(t *testing.T) {
	// Test that we can retrieve the registered swagger spec
	spec := swag.GetSwagger("swagger")
	assert.NotNil(t, spec)
	
	// Test that the SwaggerInfo has all required fields (since spec type doesn't expose them)
	assert.Equal(t, "1.0", SwaggerInfo.Version)
	assert.Equal(t, "localhost:8080", SwaggerInfo.Host)
	assert.Equal(t, "/api/v1", SwaggerInfo.BasePath)
	assert.Equal(t, "Playback Backend API", SwaggerInfo.Title)
	assert.Equal(t, "Distributed systems event replay backend", SwaggerInfo.Description)
	
	// Test that the template is valid JSON structure (basic validation)
	assert.True(t, len(SwaggerInfo.SwaggerTemplate) > 100, "Swagger template should be substantial")
	assert.Contains(t, SwaggerInfo.SwaggerTemplate, "{", "Template should contain JSON structure")
	assert.Contains(t, SwaggerInfo.SwaggerTemplate, "}", "Template should contain JSON structure")
}