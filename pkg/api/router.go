package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/openapi"
)

// Router handles HTTP routing and dispatching
type Router struct {
	routes    map[string]map[string]HandlerFunc
	spec      *openapi.Specification
	generator openapi.DataGenerator
	logger    *zap.Logger
}

// HandlerFunc represents a route handler function
type HandlerFunc func(ctx *fasthttp.RequestCtx) error

// NewRouter creates a new router instance
func NewRouter(spec *openapi.Specification, logger *zap.Logger) (*Router, error) {
	if spec == nil {
		return nil, fmt.Errorf("specification cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	router := &Router{
		routes: make(map[string]map[string]HandlerFunc),
		spec:   spec,
		logger: logger,
	}

	// Register routes from OpenAPI spec
	if err := router.loadFromSpec(); err != nil {
		return nil, fmt.Errorf("failed to load routes from spec: %w", err)
	}

	return router, nil
}

// NewRouterWithGenerator creates a new router instance with data generator
func NewRouterWithGenerator(spec *openapi.Specification, generator openapi.DataGenerator, logger *zap.Logger) (*Router, error) {
	if spec == nil {
		return nil, fmt.Errorf("specification cannot be nil")
	}
	if generator == nil {
		return nil, fmt.Errorf("generator cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	router := &Router{
		routes:    make(map[string]map[string]HandlerFunc),
		spec:      spec,
		generator: generator,
		logger:    logger,
	}

	// Register routes from OpenAPI spec with generator
	if err := router.loadFromSpecWithGenerator(); err != nil {
		return nil, fmt.Errorf("failed to load routes from spec: %w", err)
	}

	return router, nil
}

// Handler is the main FastHTTP handler
func (r *Router) Handler(ctx *fasthttp.RequestCtx) {
	method := string(ctx.Method())
	path := string(ctx.Path())

	r.logger.Debug("Handling request",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("user_agent", string(ctx.UserAgent())),
		zap.String("remote_addr", ctx.RemoteAddr().String()),
	)

	// Find matching route
	handler, params, found := r.findRoute(method, path)
	if !found {
		r.handleNotFound(ctx)
		return
	}

	// Set path parameters in context (if any)
	if len(params) > 0 {
		// Store params for handler use (simplified for now)
		r.logger.Debug("Path parameters found", zap.Any("params", params))
	}

	// Execute handler
	if err := handler(ctx); err != nil {
		r.handleError(ctx, err)
		return
	}

	r.logger.Debug("Request handled successfully",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", ctx.Response.StatusCode()),
	)
}

// loadFromSpec loads routes from OpenAPI specification
func (r *Router) loadFromSpec() error {
	for path, pathItem := range r.spec.Paths {
		if pathItem.GET != nil {
			r.registerRoute("GET", path, r.createMockHandler(path, "GET", pathItem.GET))
		}
		if pathItem.POST != nil {
			r.registerRoute("POST", path, r.createMockHandler(path, "POST", pathItem.POST))
		}
		if pathItem.PUT != nil {
			r.registerRoute("PUT", path, r.createMockHandler(path, "PUT", pathItem.PUT))
		}
		if pathItem.DELETE != nil {
			r.registerRoute("DELETE", path, r.createMockHandler(path, "DELETE", pathItem.DELETE))
		}
		if pathItem.PATCH != nil {
			r.registerRoute("PATCH", path, r.createMockHandler(path, "PATCH", pathItem.PATCH))
		}
	}

	r.logger.Info("Routes loaded from OpenAPI spec",
		zap.Int("total_routes", r.getTotalRoutes()),
	)

	return nil
}

// loadFromSpecWithGenerator loads routes from OpenAPI specification using the data generator
func (r *Router) loadFromSpecWithGenerator() error {
	for path, pathItem := range r.spec.Paths {
		if pathItem.GET != nil {
			r.registerRoute("GET", path, MockHandler(r.spec, r.generator, r.logger))
		}
		if pathItem.POST != nil {
			r.registerRoute("POST", path, MockHandler(r.spec, r.generator, r.logger))
		}
		if pathItem.PUT != nil {
			r.registerRoute("PUT", path, MockHandler(r.spec, r.generator, r.logger))
		}
		if pathItem.DELETE != nil {
			r.registerRoute("DELETE", path, MockHandler(r.spec, r.generator, r.logger))
		}
		if pathItem.PATCH != nil {
			r.registerRoute("PATCH", path, MockHandler(r.spec, r.generator, r.logger))
		}
	}

	// Add special routes
	r.registerRoute("OPTIONS", "*", OptionsHandler())
	r.registerRoute("GET", "/__health", HealthCheckHandler(r.spec))
	r.registerRoute("GET", "/__info", InfoHandler(r.spec))

	r.logger.Info("Routes loaded from OpenAPI spec with data generator",
		zap.Int("total_routes", r.getTotalRoutes()),
		zap.String("generator_type", "DefaultDataGenerator"),
	)

	return nil
}

// registerRoute registers a route with the router
func (r *Router) registerRoute(method, path string, handler HandlerFunc) {
	if r.routes[method] == nil {
		r.routes[method] = make(map[string]HandlerFunc)
	}
	r.routes[method][path] = handler

	r.logger.Debug("Route registered",
		zap.String("method", method),
		zap.String("path", path),
	)
}

// findRoute finds a matching route for the given method and path
func (r *Router) findRoute(method, path string) (HandlerFunc, map[string]string, bool) {
	methodRoutes, exists := r.routes[method]
	if !exists {
		return nil, nil, false
	}

	// Try exact match first
	if handler, exists := methodRoutes[path]; exists {
		return handler, nil, true
	}

	// Try pattern matching for parameterized paths
	for routePath, handler := range methodRoutes {
		if params := r.matchPath(routePath, path); params != nil {
			return handler, params, true
		}
	}

	return nil, nil, false
}

// matchPath matches a route path pattern against an actual path
func (r *Router) matchPath(pattern, path string) map[string]string {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil
	}

	params := make(map[string]string)
	
	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// Parameter part
			paramName := strings.Trim(part, "{}")
			params[paramName] = pathParts[i]
		} else if part != pathParts[i] {
			// Literal part that doesn't match
			return nil
		}
	}

	return params
}

// createMockHandler creates a mock handler for an operation
func (r *Router) createMockHandler(path, method string, operation *openapi.Operation) HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		// Set content type
		ctx.Response.Header.Set("Content-Type", "application/json")

		// For now, return a simple mock response
		mockResponse := map[string]interface{}{
			"message": "Mock response",
			"path":    path,
			"method":  method,
			"operationId": operation.OperationID,
		}

		// Try to find a 200 response in the spec
		if response, exists := operation.Responses["200"]; exists {
			// Check if there's an example in the response
			for mediaType, mediaObj := range response.Content {
				if mediaType == "application/json" && mediaObj.Example != nil {
					mockResponse = map[string]interface{}{
						"data": mediaObj.Example,
					}
					break
				}
			}
		}

		responseData, err := json.Marshal(mockResponse)
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}

		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(responseData)

		return nil
	}
}

// handleNotFound handles 404 responses
func (r *Router) handleNotFound(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusNotFound)
	ctx.SetContentType("application/json")
	
	response := map[string]interface{}{
		"error": "Not Found",
		"message": fmt.Sprintf("Endpoint %s %s not found", 
			string(ctx.Method()), string(ctx.Path())),
	}

	responseData, _ := json.Marshal(response)
	ctx.SetBody(responseData)

	r.logger.Warn("Route not found",
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
	)
}

// handleError handles error responses
func (r *Router) handleError(ctx *fasthttp.RequestCtx, err error) {
	ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	ctx.SetContentType("application/json")

	response := map[string]interface{}{
		"error": "Internal Server Error",
		"message": err.Error(),
	}

	responseData, _ := json.Marshal(response)
	ctx.SetBody(responseData)

	r.logger.Error("Handler error",
		zap.Error(err),
		zap.String("method", string(ctx.Method())),
		zap.String("path", string(ctx.Path())),
	)
}

// getTotalRoutes returns the total number of registered routes
func (r *Router) getTotalRoutes() int {
	total := 0
	for _, methodRoutes := range r.routes {
		total += len(methodRoutes)
	}
	return total
}