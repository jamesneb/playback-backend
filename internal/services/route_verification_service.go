package services

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

const (
	// Route verification constants
	MinRequiredRoutes       = 1
	ExpectedRouteCount      = 2 // health + traces (minimum)
	DefaultRouteMapCapacity = 50

	// Route verification field keys
	RouteFieldPath   = "path"
	RouteFieldMethod = "method"

	// Common route paths
	HealthRoutePath = "/health"
	DefaultBasePath = ""

	// HTTP methods
	MethodGET  = "GET"
	MethodPOST = "POST"

	// Route pattern matching
	TracesPathPattern  = "/traces"
	MetricsPathPattern = "/metrics"
	LogsPathPattern    = "/logs"

	// Error messages
	ErrNoRoutesRegistered = "no routes registered, server setup may have failed"
)

// RouteVerificationService handles server route setup verification
type RouteVerificationService struct {
	endpoints *api.EndpointCollection
}

// ExpectedRoute represents a route that should be registered
type ExpectedRoute struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Name   string `json:"name"`
}

// RouteVerificationResult represents the result of route verification
type RouteVerificationResult struct {
	TotalRoutes      int             `json:"total_routes"`
	ExpectedRoutes   []ExpectedRoute `json:"expected_routes"`
	MissingRoutes    []ExpectedRoute `json:"missing_routes"`
	VerifiedRoutes   []ExpectedRoute `json:"verified_routes"`
	VerificationOK   bool            `json:"verification_ok"`
	RoutesByMethod   map[string]int  `json:"routes_by_method"`
}

// NewRouteVerificationService creates a new route verification service
func NewRouteVerificationService(endpoints *api.EndpointCollection) *RouteVerificationService {
	return &RouteVerificationService{
		endpoints: endpoints,
	}
}

// VerifyServerRoutes verifies that the server has the expected routes registered
func (rvs *RouteVerificationService) VerifyServerRoutes(engine *gin.Engine) (*RouteVerificationResult, error) {
	// Get all registered routes
	routes := engine.Routes()
	if len(routes) < MinRequiredRoutes {
		return &RouteVerificationResult{
			TotalRoutes:    len(routes),
			VerificationOK: false,
		}, errors.New(ErrNoRoutesRegistered)
	}

	// Define expected routes
	expectedRoutes := rvs.buildExpectedRoutes()

	// Build route lookup set for efficient O(1) access
	routeSet := rvs.buildRouteSet(routes)

	// Verify each expected route
	verificationResult := rvs.performRouteVerification(expectedRoutes, routeSet)
	verificationResult.TotalRoutes = len(routes)
	verificationResult.RoutesByMethod = rvs.countRoutesByMethod(routes)

	// Log verification results
	rvs.logVerificationResults(verificationResult)

	return verificationResult, nil
}

// buildExpectedRoutes constructs the list of routes that should be present using direct assertions
func (rvs *RouteVerificationService) buildExpectedRoutes() []ExpectedRoute {
	expectedRoutes := make([]ExpectedRoute, 0, ExpectedRouteCount)

	// Always expect health check route - direct assertion
	expectedRoutes = append(expectedRoutes, ExpectedRoute{
		Path:   HealthRoutePath,
		Method: MethodGET,
		Name:   "health_check",
	})

	// Add API routes if endpoints are configured - direct assertions
	if rvs.endpoints != nil {
		// Traces endpoints
		if tracesPath := rvs.endpoints.TracesRelative(); tracesPath != DefaultBasePath {
			expectedRoutes = append(expectedRoutes,
				ExpectedRoute{
					Path:   tracesPath,
					Method: MethodPOST,
					Name:   "create_trace",
				},
				ExpectedRoute{
					Path:   rvs.endpoints.TraceByIDRelative(),
					Method: MethodGET,
					Name:   "get_trace",
				})
		}

		// Metrics endpoints
		if metricsPath := rvs.endpoints.MetricsRelative(); metricsPath != DefaultBasePath {
			expectedRoutes = append(expectedRoutes,
				ExpectedRoute{
					Path:   metricsPath,
					Method: MethodPOST,
					Name:   "create_metrics",
				},
				ExpectedRoute{
					Path:   metricsPath,
					Method: MethodGET,
					Name:   "get_metrics",
				})
		}

		// Logs endpoints
		if logsPath := rvs.endpoints.LogsRelative(); logsPath != DefaultBasePath {
			expectedRoutes = append(expectedRoutes,
				ExpectedRoute{
					Path:   logsPath,
					Method: MethodPOST,
					Name:   "create_logs",
				},
				ExpectedRoute{
					Path:   logsPath,
					Method: MethodGET,
					Name:   "get_logs",
				})
		}
	}

	return expectedRoutes
}

// buildRouteKey creates a unique key for route lookup using path+method
func (rvs *RouteVerificationService) buildRouteKey(path, method string) string {
	return method + ":" + path
}

// buildRouteSet creates a map for efficient route lookup with method-specific keys
func (rvs *RouteVerificationService) buildRouteSet(routes gin.RoutesInfo) map[string]bool {
	capacity := len(routes)
	if capacity < DefaultRouteMapCapacity {
		capacity = DefaultRouteMapCapacity
	}
	routeSet := make(map[string]bool, capacity)

	// Build route lookup set with method+path keys for exact matching
	for _, route := range routes {
		routeKey := rvs.buildRouteKey(route.Path, route.Method)
		routeSet[routeKey] = true
	}

	return routeSet
}

// performRouteVerification checks each expected route against registered routes using direct assertions
func (rvs *RouteVerificationService) performRouteVerification(expectedRoutes []ExpectedRoute, routeSet map[string]bool) *RouteVerificationResult {
	result := &RouteVerificationResult{
		ExpectedRoutes: expectedRoutes,
		MissingRoutes:  make([]ExpectedRoute, 0),
		VerifiedRoutes: make([]ExpectedRoute, 0),
		VerificationOK: true,
		RoutesByMethod: make(map[string]int),
	}

	// Direct assertion - check each expected route with exact path+method matching
	for _, expectedRoute := range expectedRoutes {
		routeKey := rvs.buildRouteKey(expectedRoute.Path, expectedRoute.Method)
		if routeSet[routeKey] {
			result.VerifiedRoutes = append(result.VerifiedRoutes, expectedRoute)
		} else {
			result.MissingRoutes = append(result.MissingRoutes, expectedRoute)
			result.VerificationOK = false
		}
	}

	return result
}

// countRoutesByMethod provides route statistics by HTTP method
func (rvs *RouteVerificationService) countRoutesByMethod(routes gin.RoutesInfo) map[string]int {
	methodCounts := make(map[string]int)
	for _, route := range routes {
		methodCounts[route.Method]++
	}
	return methodCounts
}


// logVerificationResults logs the results of route verification
func (rvs *RouteVerificationService) logVerificationResults(result *RouteVerificationResult) {
	if result.VerificationOK {
		verifiedRouteNames := make([]string, len(result.VerifiedRoutes))
		for i, route := range result.VerifiedRoutes {
			verifiedRouteNames[i] = route.Name
		}
		logger.Info("Server route verification completed successfully",
			zap.Int("total_routes", result.TotalRoutes),
			zap.Strings("verified_routes", verifiedRouteNames))
	} else {
		missingRouteNames := make([]string, len(result.MissingRoutes))
		for i, route := range result.MissingRoutes {
			missingRouteNames[i] = route.Name
		}
		verifiedRouteNames := make([]string, len(result.VerifiedRoutes))
		for i, route := range result.VerifiedRoutes {
			verifiedRouteNames[i] = route.Name
		}

		logger.Warn("Server route verification found missing routes",
			zap.Int("total_routes", result.TotalRoutes),
			zap.Strings("missing_routes", missingRouteNames),
			zap.Strings("verified_routes", verifiedRouteNames))

		// Log individual missing routes for debugging
		for _, missingRoute := range result.MissingRoutes {
			logger.Warn("Expected route not found",
				zap.String("method", missingRoute.Method),
				zap.String("path", missingRoute.Path),
				zap.String("name", missingRoute.Name))
		}
	}
}

// GetRouteStats returns basic statistics about the registered routes
func (rvs *RouteVerificationService) GetRouteStats(engine *gin.Engine) map[string]interface{} {
	routes := engine.Routes()

	// Count routes by method
	methodCounts := make(map[string]int)
	for _, route := range routes {
		methodCounts[route.Method]++
	}

	return map[string]interface{}{
		"total_routes":   len(routes),
		"method_counts":  methodCounts,
		"gin_mode":       gin.Mode(),
	}
}