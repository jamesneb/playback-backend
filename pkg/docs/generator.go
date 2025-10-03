package docs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jamesneb/playback-backend/pkg/config"
)

// OpenAPISpec represents a complete OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Servers    []APIServer         `json:"servers"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
	Tags       []Tag               `json:"tags"`
}

// Info contains API metadata
type Info struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Version        string  `json:"version"`
	TermsOfService string  `json:"termsOfService,omitempty"`
	Contact        Contact `json:"contact,omitempty"`
	License        License `json:"license,omitempty"`
}

// Contact contains contact information
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License contains license information
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// APIServer represents an API server
type APIServer struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// PathItem represents operations on a path
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Options *Operation `json:"options,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
}

// Operation represents an API operation
type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
}

// Parameter represents an operation parameter
type Parameter struct {
	Name            string      `json:"name"`
	In              string      `json:"in"`
	Description     string      `json:"description,omitempty"`
	Required        bool        `json:"required,omitempty"`
	Deprecated      bool        `json:"deprecated,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty"`
	Schema          *Schema     `json:"schema,omitempty"`
	Example         interface{} `json:"example,omitempty"`
}

// RequestBody represents a request body
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
	Required    bool                 `json:"required,omitempty"`
}

// Response represents an API response
type Response struct {
	Ref         string               `json:"$ref,omitempty"`
	Description string               `json:"description,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// MediaType represents a media type object
type MediaType struct {
	Schema   *Schema     `json:"schema,omitempty"`
	Example  interface{} `json:"example,omitempty"`
	Examples interface{} `json:"examples,omitempty"`
}

// Header represents a response header
type Header struct {
	Description string      `json:"description,omitempty"`
	Schema      *Schema     `json:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty"`
}

// Components holds reusable components
type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	Responses       map[string]Response       `json:"responses,omitempty"`
	Parameters      map[string]Parameter      `json:"parameters,omitempty"`
	RequestBodies   map[string]RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// Schema represents a JSON Schema
type Schema struct {
	Ref                  string             `json:"$ref,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              interface{}        `json:"default,omitempty"`
	Example              interface{}        `json:"example,omitempty"`
	Enum                 []interface{}      `json:"enum,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties interface{}        `json:"additionalProperties,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	AllOf                []*Schema          `json:"allOf,omitempty"`
	Not                  *Schema            `json:"not,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	MaxItems             *int               `json:"maxItems,omitempty"`
	UniqueItems          bool               `json:"uniqueItems,omitempty"`
}

// Tag represents an API tag
type Tag struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

// ExternalDocs represents external documentation
type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// SecurityScheme represents a security scheme
type SecurityScheme struct {
	Type             string      `json:"type"`
	Description      string      `json:"description,omitempty"`
	Name             string      `json:"name,omitempty"`
	In               string      `json:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty"`
	Flows            interface{} `json:"flows,omitempty"`
	OpenIDConnectURL string      `json:"openIdConnectUrl,omitempty"`
}

// SecurityRequirement represents a security requirement
type SecurityRequirement map[string][]string

// Generator generates OpenAPI documentation
type Generator struct {
	cfg  *config.ConsolidatedConfig
	spec *OpenAPISpec
}

// NewGenerator creates a new documentation generator
func NewGenerator(cfg *config.ConsolidatedConfig) *Generator {
	return &Generator{
		cfg: cfg,
		spec: &OpenAPISpec{
			OpenAPI: "3.0.3",
			Info: Info{
				Title:       "Playback Backend API",
				Description: "High-performance OpenTelemetry data ingestion and query API for distributed systems observability",
				Version:     cfg.App.Version,
				Contact: Contact{
					Name:  "Playback Team",
					Email: "support@playback.com",
				},
				License: License{
					Name: "MIT",
					URL:  "https://opensource.org/licenses/MIT",
				},
			},
			Paths: make(map[string]PathItem),
			Components: Components{
				Schemas:         make(map[string]Schema),
				Responses:       make(map[string]Response),
				SecuritySchemes: make(map[string]SecurityScheme),
			},
			Tags: []Tag{
				{
					Name:        "traces",
					Description: "OpenTelemetry trace data ingestion and querying",
				},
				{
					Name:        "metrics",
					Description: "OpenTelemetry metrics data ingestion and querying",
				},
				{
					Name:        "logs",
					Description: "OpenTelemetry logs data ingestion and querying",
				},
				{
					Name:        "replay",
					Description: "Session replay data storage and retrieval",
				},
				{
					Name:        "health",
					Description: "Service health and monitoring endpoints",
				},
			},
		},
	}
}

// GenerateSpec creates the complete OpenAPI specification
func (g *Generator) GenerateSpec() (*OpenAPISpec, error) {
	// Add servers based on configuration
	g.addServers()

	// Add security schemes
	g.addSecuritySchemes()

	// Add common components
	g.addCommonSchemas()

	// Add API paths
	g.addTracePaths()
	g.addMetricsPaths()
	g.addLogsPaths()
	g.addReplayPaths()
	g.addHealthPaths()

	// Add error responses
	g.addErrorResponses()

	return g.spec, nil
}

// addServers adds server configurations
func (g *Generator) addServers() {
	baseURL := fmt.Sprintf("http://%s:%d", g.cfg.Network.HTTP.Host, g.cfg.Network.HTTP.Port)
	if g.cfg.App.Environment == "production" {
		baseURL = "https://api.playback.com"
	}

	g.spec.Servers = []APIServer{
		{
			URL:         baseURL,
			Description: fmt.Sprintf("%s environment", g.cfg.App.Environment),
		},
	}

	// Add additional servers for different environments
	if g.cfg.App.Environment == "development" {
		g.spec.Servers = append(g.spec.Servers,
			APIServer{
				URL:         "https://staging-api.playback.com",
				Description: "Staging environment",
			},
			APIServer{
				URL:         "https://api.playbook.com",
				Description: "Production environment",
			},
		)
	}
}

// addSecuritySchemes adds authentication schemes
func (g *Generator) addSecuritySchemes() {
	g.spec.Components.SecuritySchemes = map[string]SecurityScheme{
		"BearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT Bearer token authentication",
		},
		"ApiKeyAuth": {
			Type:        "apiKey",
			In:          "header",
			Name:        "X-API-Key",
			Description: "API key authentication",
		},
	}
}

// addCommonSchemas adds reusable schema components
func (g *Generator) addCommonSchemas() {
	// Standard error response schema
	g.spec.Components.Schemas["ErrorResponse"] = g.generateErrorSchema()

	// Validation error schema
	g.spec.Components.Schemas["ValidationError"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"field": {
				Type:        "string",
				Description: "The field that failed validation",
				Example:     "email",
			},
			"rule": {
				Type:        "string",
				Description: "The validation rule that failed",
				Example:     "email",
			},
			"message": {
				Type:        "string",
				Description: "Human-readable error message",
				Example:     "must be a valid email address",
			},
			"value": {
				Description: "The value that failed validation",
				Example:     "invalid-email",
			},
		},
		Required: []string{"field", "rule", "message"},
	}

	// OpenTelemetry trace schema
	g.spec.Components.Schemas["TraceData"] = Schema{
		Type:        "object",
		Description: "OpenTelemetry trace data in OTLP format",
		Properties: map[string]*Schema{
			"resourceSpans": {
				Type: "array",
				Items: &Schema{
					Ref: "#/components/schemas/ResourceSpan",
				},
			},
		},
		Example: map[string]interface{}{
			"resourceSpans": []map[string]interface{}{
				{
					"resource": map[string]interface{}{
						"attributes": []map[string]interface{}{
							{
								"key": "service.name",
								"value": map[string]interface{}{
									"stringValue": "user-service",
								},
							},
						},
					},
				},
			},
		},
	}

	// Add more OTLP schemas
	g.addOTLPSchemas()
}

// generateErrorSchema creates the standardized error response schema
func (g *Generator) generateErrorSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Standardized error response format",
		Properties: map[string]*Schema{
			"error": {
				Type: "object",
				Properties: map[string]*Schema{
					"code": {
						Type:        "string",
						Description: "Machine-readable error code",
						Enum: []interface{}{
							"BAD_REQUEST", "UNAUTHORIZED", "FORBIDDEN", "NOT_FOUND",
							"CONFLICT", "VALIDATION_FAILED", "RATE_LIMITED",
							"REQUEST_TOO_LARGE", "UNSUPPORTED_MEDIA_TYPE",
							"INTERNAL_SERVER_ERROR", "SERVICE_UNAVAILABLE",
							"DATABASE_ERROR", "EXTERNAL_SERVICE_ERROR",
							"TIMEOUT", "CIRCUIT_BREAKER_OPEN",
						},
						Example: "VALIDATION_FAILED",
					},
					"message": {
						Type:        "string",
						Description: "Human-readable error message",
						Example:     "Request validation failed",
					},
					"category": {
						Type:        "string",
						Description: "Error category for classification",
						Enum:        []interface{}{"client", "server", "system", "business"},
						Example:     "client",
					},
					"details": {
						Type:                 "object",
						Description:          "Additional error details",
						AdditionalProperties: true,
						Example: map[string]interface{}{
							"field":          "email",
							"provided_value": "invalid-email",
						},
					},
					"validation": {
						Type: "array",
						Items: &Schema{
							Ref: "#/components/schemas/ValidationError",
						},
						Description: "Field-specific validation errors",
					},
					"cause": {
						Description: "Underlying error cause",
						Ref:         "#/components/schemas/ErrorCause",
					},
					"retryable": {
						Type:        "boolean",
						Description: "Whether the operation can be retried",
						Example:     true,
					},
					"retry_after": {
						Type:        "integer",
						Description: "Seconds to wait before retrying",
						Example:     30,
					},
				},
				Required: []string{"code", "message", "category"},
			},
			"request_id": {
				Type:        "string",
				Description: "Unique request identifier for tracing",
				Example:     "req_01HQWE123ABC",
			},
			"timestamp": {
				Type:        "string",
				Format:      "date-time",
				Description: "ISO 8601 timestamp when error occurred",
				Example:     time.Now().UTC().Format(time.RFC3339),
			},
			"path": {
				Type:        "string",
				Description: "API path where error occurred",
				Example:     "/api/v1/traces",
			},
		},
		Required: []string{"error", "timestamp"},
		Example: map[string]interface{}{
			"error": map[string]interface{}{
				"code":      "VALIDATION_FAILED",
				"message":   "Request validation failed",
				"category":  "client",
				"retryable": false,
				"validation": []map[string]interface{}{
					{
						"field":   "email",
						"rule":    "email",
						"message": "must be a valid email address",
						"value":   "invalid-email",
					},
				},
			},
			"request_id": "req_01HQWE123ABC",
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"path":       "/api/v1/traces",
		},
	}
}

// addOTLPSchemas adds OpenTelemetry Protocol schemas
func (g *Generator) addOTLPSchemas() {
	// Resource span schema
	g.spec.Components.Schemas["ResourceSpan"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"resource": {
				Ref: "#/components/schemas/Resource",
			},
			"scopeSpans": {
				Type: "array",
				Items: &Schema{
					Ref: "#/components/schemas/ScopeSpan",
				},
			},
		},
	}

	// Resource schema
	g.spec.Components.Schemas["Resource"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"attributes": {
				Type: "array",
				Items: &Schema{
					Ref: "#/components/schemas/KeyValue",
				},
			},
		},
	}

	// Key-value attribute schema
	g.spec.Components.Schemas["KeyValue"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"key": {
				Type:        "string",
				Description: "Attribute key",
				Example:     "service.name",
			},
			"value": {
				Ref: "#/components/schemas/AnyValue",
			},
		},
		Required: []string{"key", "value"},
	}

	// Any value schema (supports string, int, double, bool)
	g.spec.Components.Schemas["AnyValue"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"stringValue": {Type: "string"},
			"intValue":    {Type: "integer"},
			"doubleValue": {Type: "number"},
			"boolValue":   {Type: "boolean"},
		},
	}
}

// addTracePaths adds trace-related API paths
func (g *Generator) addTracePaths() {
	// POST /api/v1/traces
	g.spec.Paths["/api/v1/traces"] = PathItem{
		Post: &Operation{
			Tags:        []string{"traces"},
			Summary:     "Ingest trace data",
			Description: "Accept OpenTelemetry trace data in OTLP format for high-performance ingestion",
			OperationID: "ingestTraces",
			RequestBody: &RequestBody{
				Description: "OpenTelemetry trace data",
				Required:    true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Ref: "#/components/schemas/TraceData",
						},
					},
				},
			},
			Responses: map[string]Response{
				"201": {
					Description: "Trace data successfully ingested",
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"id": {
										Type:        "string",
										Description: "Unique trace identifier",
										Example:     "trace_01HQWE123ABC",
									},
									"trace_id": {
										Type:        "string",
										Description: "OpenTelemetry trace ID",
										Example:     "1234567890abcdef1234567890abcdef",
									},
									"status": {
										Type:    "string",
										Example: "accepted",
									},
									"created_at": {
										Type:   "string",
										Format: "date-time",
									},
								},
							},
						},
					},
				},
				"400": {Ref: "#/components/responses/BadRequest"},
				"413": {Ref: "#/components/responses/PayloadTooLarge"},
				"429": {Ref: "#/components/responses/RateLimited"},
				"503": {Ref: "#/components/responses/ServiceUnavailable"},
			},
		},
	}

	// GET /api/v1/traces/{id}
	g.spec.Paths["/api/v1/traces/{id}"] = PathItem{
		Get: &Operation{
			Tags:        []string{"traces"},
			Summary:     "Get trace by ID",
			Description: "Retrieve trace information by trace ID from ClickHouse storage",
			OperationID: "getTrace",
			Parameters: []Parameter{
				{
					Name:        "id",
					In:          "path",
					Description: "Trace ID to retrieve",
					Required:    true,
					Schema: &Schema{
						Type:    "string",
						Pattern: "^[a-fA-F0-9]{32}$",
					},
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "Trace found",
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"trace_id":   {Type: "string"},
									"start_time": {Type: "string", Format: "date-time"},
									"duration":   {Type: "integer", Description: "Duration in nanoseconds"},
									"span_count": {Type: "integer"},
									"status":     {Type: "string"},
								},
							},
						},
					},
				},
				"404": {Ref: "#/components/responses/NotFound"},
				"503": {Ref: "#/components/responses/ServiceUnavailable"},
			},
		},
	}
}

// addMetricsPaths adds metrics-related API paths
func (g *Generator) addMetricsPaths() {
	// Similar structure for metrics endpoints
	g.spec.Paths["/api/v1/metrics"] = PathItem{
		Post: &Operation{
			Tags:        []string{"metrics"},
			Summary:     "Ingest metrics data",
			Description: "Accept OpenTelemetry metrics data in OTLP format",
			OperationID: "ingestMetrics",
			RequestBody: &RequestBody{
				Description: "OpenTelemetry metrics data",
				Required:    true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Type:        "object",
							Description: "OTLP metrics payload",
						},
					},
				},
			},
			Responses: map[string]Response{
				"201": {Description: "Metrics data successfully ingested"},
				"400": {Ref: "#/components/responses/BadRequest"},
				"413": {Ref: "#/components/responses/PayloadTooLarge"},
				"429": {Ref: "#/components/responses/RateLimited"},
			},
		},
		Get: &Operation{
			Tags:        []string{"metrics"},
			Summary:     "Query metrics data",
			Description: "Query aggregated metrics from ClickHouse",
			OperationID: "queryMetrics",
			Parameters: []Parameter{
				{
					Name:        "service",
					In:          "query",
					Description: "Filter by service name",
					Schema:      &Schema{Type: "string"},
				},
				{
					Name:        "start_time",
					In:          "query",
					Description: "Start time for query range",
					Schema:      &Schema{Type: "string", Format: "date-time"},
				},
				{
					Name:        "end_time",
					In:          "query",
					Description: "End time for query range",
					Schema:      &Schema{Type: "string", Format: "date-time"},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Metrics query results"},
				"400": {Ref: "#/components/responses/BadRequest"},
			},
		},
	}
}

// addLogsPaths adds logs-related API paths
func (g *Generator) addLogsPaths() {
	g.spec.Paths["/api/v1/logs"] = PathItem{
		Post: &Operation{
			Tags:        []string{"logs"},
			Summary:     "Ingest log data",
			Description: "Accept OpenTelemetry log data in OTLP format",
			OperationID: "ingestLogs",
			RequestBody: &RequestBody{
				Description: "OpenTelemetry log data",
				Required:    true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{Type: "object"},
					},
				},
			},
			Responses: map[string]Response{
				"201": {Description: "Log data successfully ingested"},
				"400": {Ref: "#/components/responses/BadRequest"},
			},
		},
	}
}

// addReplayPaths adds replay-related API paths
func (g *Generator) addReplayPaths() {
	g.spec.Paths["/api/v1/replay/sessions"] = PathItem{
		Post: &Operation{
			Tags:        []string{"replay"},
			Summary:     "Upload session replay",
			Description: "Upload session replay data to S3 storage",
			OperationID: "uploadReplay",
			Responses: map[string]Response{
				"201": {Description: "Replay uploaded successfully"},
				"400": {Ref: "#/components/responses/BadRequest"},
			},
		},
	}

	g.spec.Paths["/api/v1/replay/sessions/{id}"] = PathItem{
		Get: &Operation{
			Tags:        []string{"replay"},
			Summary:     "Get session replay",
			Description: "Retrieve session replay data from S3",
			OperationID: "getReplay",
			Parameters: []Parameter{
				{
					Name:        "id",
					In:          "path",
					Description: "Session replay ID",
					Required:    true,
					Schema:      &Schema{Type: "string"},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Replay data"},
				"404": {Ref: "#/components/responses/NotFound"},
			},
		},
	}
}

// addHealthPaths adds health check paths
func (g *Generator) addHealthPaths() {
	g.spec.Paths["/health"] = PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Health check",
			Description: "Get service health status and dependencies",
			OperationID: "healthCheck",
			Responses: map[string]Response{
				"200": {
					Description: "Service is healthy",
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Type: "object",
								Properties: map[string]*Schema{
									"status":      {Type: "string", Example: "healthy"},
									"version":     {Type: "string"},
									"environment": {Type: "string"},
									"dependencies": {
										Type: "object",
										AdditionalProperties: &Schema{
											Type: "object",
											Properties: map[string]*Schema{
												"status":  {Type: "string"},
												"latency": {Type: "string"},
											},
										},
									},
								},
							},
						},
					},
				},
				"503": {
					Description: "Service is unhealthy",
					Content: map[string]MediaType{
						"application/json": {
							Schema: &Schema{
								Ref: "#/components/schemas/ErrorResponse",
							},
						},
					},
				},
			},
		},
	}
}

// addErrorResponses adds reusable error response components
func (g *Generator) addErrorResponses() {
	g.spec.Components.Responses = map[string]Response{
		"BadRequest": {
			Description: "Bad Request - Invalid input parameters",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
		},
		"NotFound": {
			Description: "Not Found - Requested resource not found",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
		},
		"RateLimited": {
			Description: "Too Many Requests - Rate limit exceeded",
			Headers: map[string]Header{
				"Retry-After": {
					Description: "Seconds to wait before retrying",
					Schema:      &Schema{Type: "integer"},
				},
			},
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
		},
		"PayloadTooLarge": {
			Description: "Payload Too Large - Request exceeds size limits",
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
		},
		"ServiceUnavailable": {
			Description: "Service Unavailable - Temporary service failure",
			Headers: map[string]Header{
				"Retry-After": {
					Description: "Seconds to wait before retrying",
					Schema:      &Schema{Type: "integer"},
				},
			},
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: "#/components/schemas/ErrorResponse",
					},
				},
			},
		},
	}

	// Add error cause schema
	g.spec.Components.Schemas["ErrorCause"] = Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"code": {
				Type:        "string",
				Description: "Error code of the underlying cause",
			},
			"message": {
				Type:        "string",
				Description: "Message of the underlying cause",
			},
			"category": {
				Type:        "string",
				Description: "Category of the underlying cause",
			},
		},
	}
}

// ToJSON converts the OpenAPI spec to JSON
func (g *Generator) ToJSON() ([]byte, error) {
	spec, err := g.GenerateSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to generate spec: %w", err)
	}

	return json.MarshalIndent(spec, "", "  ")
}
