package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/seb7887/vanta/pkg/config"
	"github.com/seb7887/vanta/pkg/openapi"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// MockHandler handles requests by generating mock responses based on OpenAPI specification
func MockHandler(spec *openapi.Specification, generator openapi.DataGenerator, logger *zap.Logger) HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		method := string(ctx.Method())
		path := string(ctx.Path())

		logger.Debug("Processing mock request",
			zap.String("method", method),
			zap.String("path", path),
		)

		// Find matching endpoint in the OpenAPI spec
		endpoint, pathParams, found := findMatchingEndpoint(spec, method, path)
		if !found {
			return handleEndpointNotFound(ctx, method, path, logger)
		}

		logger.Debug("Found matching endpoint",
			zap.String("operation_id", endpoint.OperationID),
			zap.Any("path_params", pathParams),
		)

		// Determine appropriate response status code
		responseCode := determineResponseCode(endpoint)

		// Get response schema for the status code
		responseSchema, mediaType, mediaTypeObj := getResponseSchemaWithMediaType(endpoint, responseCode)
		if responseSchema == nil {
			return handleNoResponseSchema(ctx, responseCode, logger)
		}

		rawRequestedExample := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Mock-Example")))

		// Copy examples from MediaTypeObject to Schema if available
		if mediaTypeObj != nil && len(mediaTypeObj.Examples) > 0 {
			logger.Debug("Found examples in MediaTypeObject",
				zap.Int("example_count", len(mediaTypeObj.Examples)),
				zap.String("requested_example", rawRequestedExample),
			)

			if responseSchema.Examples == nil {
				responseSchema.Examples = make(map[string]openapi.ExampleObject)
			}
			for name, example := range mediaTypeObj.Examples {
				responseSchema.Examples[name] = example
				logger.Debug("Added example to schema",
					zap.String("example_name", name),
					zap.Any("example_value", example.Value),
				)
			}
		} else {
			logger.Debug("No examples found in MediaTypeObject")
		}

		availableExamples := 0
		if responseSchema.Examples != nil {
			availableExamples = len(responseSchema.Examples)
		}
		selection := resolveExampleSelection(rawRequestedExample, responseSchema, "header", "")
		if selection.HeaderProvided && !selection.HeaderMatched {
			logger.Warn("Requested example not found; falling back",
				zap.String("requested_example", rawRequestedExample),
				zap.String("selection_source", selection.Source),
				zap.Int("available_examples", availableExamples),
			)
		}
		logger.Debug("Example selection applied",
			zap.String("raw_requested_example", rawRequestedExample),
			zap.String("effective_example", selection.Requested),
			zap.String("selection_source", selection.Source),
			zap.Int("available_examples", availableExamples),
		)

		requestedExample := selection.Requested

		logger.Debug("Generating mock response",
			zap.String("response_code", responseCode),
			zap.String("media_type", mediaType),
		)

		// Create generation context
		var seed int64
		if defaultGen, ok := generator.(*openapi.DefaultDataGenerator); ok {
			seed = defaultGen.GetSeed()
		} else {
			seed = ctx.Time().UnixNano()
		}

		genCtx := &openapi.GenerationContext{
			MaxDepth:         5,
			CurrentDepth:     0,
			Visited:          make(map[string]bool),
			ArraySizes:       make(map[string]int),
			Locale:           "en",
			Seed:             seed,
			Timestamp:        ctx.Time(),
			RequestedExample: requestedExample,
		}

		// Generate mock data
		mockData, err := generator.Generate(responseSchema, genCtx)
		if err != nil {
			logger.Error("Failed to generate mock data", zap.Error(err))
			return handleGenerationError(ctx, err, logger)
		}

		// Set response headers
		setResponseHeaders(ctx, responseCode, mediaType)

		// Serialize and send response
		return sendMockResponse(ctx, mockData, logger)
	}
}

// MockHandlerWithConfig handles requests by generating mock responses with configuration
func MockHandlerWithConfig(spec *openapi.Specification, generator openapi.DataGenerator, mockConfig config.MockConfig, logger *zap.Logger) HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		method := string(ctx.Method())
		path := string(ctx.Path())

		// Extract requested example and normalize strategy/defaults
		rawRequestedExample := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Mock-Example")))
		normalizedStrategy := normalizeExampleStrategy(mockConfig.ExampleStrategy)
		defaultExample := strings.TrimSpace(mockConfig.DefaultExample)

		logger.Debug("Processing mock request with config",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int64("seed", mockConfig.Seed),
			zap.String("locale", mockConfig.Locale),
			zap.Int("max_depth", mockConfig.MaxDepth),
			zap.Int("default_array_size", mockConfig.DefaultArraySize),
			zap.Bool("prefer_examples", mockConfig.PreferExamples),
			zap.String("requested_example", rawRequestedExample),
			zap.String("example_strategy", normalizedStrategy),
			zap.String("default_example", defaultExample),
		)

		// Find matching endpoint in the OpenAPI spec
		endpoint, pathParams, found := findMatchingEndpoint(spec, method, path)
		if !found {
			return handleEndpointNotFound(ctx, method, path, logger)
		}

		logger.Debug("Found matching endpoint",
			zap.String("operation_id", endpoint.OperationID),
			zap.Any("path_params", pathParams),
		)

		// Determine appropriate response status code
		responseCode := determineResponseCode(endpoint)

		// Get response schema for the status code
		responseSchema, mediaType, mediaTypeObj := getResponseSchemaWithMediaType(endpoint, responseCode)
		if responseSchema == nil {
			return handleNoResponseSchema(ctx, responseCode, logger)
		}

		// Copy examples from MediaTypeObject to Schema if available
		if mediaTypeObj != nil && len(mediaTypeObj.Examples) > 0 {
			logger.Debug("Found examples in MediaTypeObject",
				zap.Int("example_count", len(mediaTypeObj.Examples)),
				zap.String("requested_example", rawRequestedExample),
			)

			if responseSchema.Examples == nil {
				responseSchema.Examples = make(map[string]openapi.ExampleObject)
			}
			for name, example := range mediaTypeObj.Examples {
				responseSchema.Examples[name] = example
				logger.Debug("Added example to schema",
					zap.String("example_name", name),
					zap.Any("example_value", example.Value),
				)
			}
		} else {
			logger.Debug("No examples found in MediaTypeObject")
		}

		availableExamples := 0
		if responseSchema.Examples != nil {
			availableExamples = len(responseSchema.Examples)
		}
		selection := resolveExampleSelection(rawRequestedExample, responseSchema, normalizedStrategy, defaultExample)
		if selection.HeaderProvided && !selection.HeaderMatched {
			logger.Warn("Requested example not found; falling back",
				zap.String("requested_example", rawRequestedExample),
				zap.String("selection_source", selection.Source),
				zap.Int("available_examples", availableExamples),
			)
		}
		logger.Debug("Example selection applied",
			zap.String("raw_requested_example", rawRequestedExample),
			zap.String("effective_example", selection.Requested),
			zap.String("selection_source", selection.Source),
			zap.Int("available_examples", availableExamples),
		)

		requestedExample := selection.Requested

		logger.Debug("Generating mock response with config",
			zap.String("response_code", responseCode),
			zap.String("media_type", mediaType),
		)

		// Create generation context with configuration
		var seed int64 = mockConfig.Seed
		if seed == 0 {
			if defaultGen, ok := generator.(*openapi.DefaultDataGenerator); ok {
				seed = defaultGen.GetSeed()
			} else {
				seed = ctx.Time().UnixNano()
			}
		}

		genCtx := openapi.NewGenerationContextWithExample(
			mockConfig.MaxDepth,
			seed,
			mockConfig.Locale,
			mockConfig.DefaultArraySize,
			mockConfig.PreferExamples,
			requestedExample,
		)
		genCtx.Timestamp = ctx.Time()

		// Generate mock data
		mockData, err := generator.Generate(responseSchema, genCtx)
		if err != nil {
			logger.Error("Failed to generate mock data", zap.Error(err))
			return handleGenerationError(ctx, err, logger)
		}

		// Set response headers
		setResponseHeaders(ctx, responseCode, mediaType)

		// Serialize and send response
		return sendMockResponse(ctx, mockData, logger)
	}
}

type exampleSelectionResult struct {
	Requested      string
	Source         string
	HeaderProvided bool
	HeaderMatched  bool
}

func resolveExampleSelection(rawHeader string, schema *openapi.Schema, strategy string, defaultExample string) exampleSelectionResult {
	result := exampleSelectionResult{Source: "no_examples"}

	if schema == nil {
		return result
	}

	trimmedHeader := strings.TrimSpace(rawHeader)
	trimmedDefault := strings.TrimSpace(defaultExample)
	strategyValue := normalizeExampleStrategy(strategy)

	result.HeaderProvided = trimmedHeader != ""

	var examples map[string]openapi.ExampleObject
	if len(schema.Examples) > 0 {
		examples = schema.Examples
	}

	if len(examples) > 0 {
		result.Source = "fallback_first"
	}

	if trimmedHeader != "" && len(examples) > 0 {
		if strings.EqualFold(trimmedHeader, "random") {
			result.Requested = "random"
			result.Source = "header_random"
			result.HeaderMatched = true
			return result
		}

		if _, exists := examples[trimmedHeader]; exists {
			result.Requested = trimmedHeader
			result.Source = "header"
			result.HeaderMatched = true
			return result
		}
	}

	if strategyValue == "header" {
		if trimmedDefault != "" && len(examples) > 0 {
			if _, exists := examples[trimmedDefault]; exists {
				result.Requested = trimmedDefault
				result.Source = "config_default"
				return result
			}
		}
	} else if strategyValue == "random" {
		if len(examples) > 0 {
			result.Requested = "random"
			result.Source = "config_random"
			return result
		}
	} else if strategyValue == "first" {
		if len(examples) > 0 {
			first := firstExampleName(examples)
			result.Requested = first
			result.Source = "config_first"
			return result
		}
	}

	if len(examples) > 0 {
		first := firstExampleName(examples)
		result.Requested = first
		if result.Source == "no_examples" {
			result.Source = "fallback_first"
		}
		return result
	}

	if schema.Example != nil {
		result.Source = "single_example"
	} else {
		result.Source = "no_examples"
	}

	return result
}

func normalizeExampleStrategy(strategy string) string {
	s := strings.ToLower(strings.TrimSpace(strategy))
	switch s {
	case "", "header":
		return "header"
	case "first":
		return "first"
	case "random":
		return "random"
	default:
		return "header"
	}
}

func firstExampleName(examples map[string]openapi.ExampleObject) string {
	if len(examples) == 0 {
		return ""
	}

	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0]
}

// findMatchingEndpoint finds the OpenAPI endpoint that matches the request
func findMatchingEndpoint(spec *openapi.Specification, method, path string) (*openapi.Operation, map[string]string, bool) {
	// Try exact path match first
	if pathItem, exists := spec.Paths[path]; exists {
		if operation := getOperationFromPathItem(pathItem, method); operation != nil {
			return operation, nil, true
		}
	}

	// Try parameterized path matching
	for specPath, pathItem := range spec.Paths {
		if params := matchParameterizedPath(specPath, path); params != nil {
			if operation := getOperationFromPathItem(pathItem, method); operation != nil {
				return operation, params, true
			}
		}
	}

	return nil, nil, false
}

// getOperationFromPathItem gets the operation for a specific HTTP method
func getOperationFromPathItem(pathItem openapi.PathItem, method string) *openapi.Operation {
	switch strings.ToUpper(method) {
	case "GET":
		return pathItem.GET
	case "POST":
		return pathItem.POST
	case "PUT":
		return pathItem.PUT
	case "DELETE":
		return pathItem.DELETE
	case "PATCH":
		return pathItem.PATCH
	default:
		return nil
	}
}

// matchParameterizedPath matches a parameterized OpenAPI path against an actual request path
func matchParameterizedPath(specPath, requestPath string) map[string]string {
	specParts := strings.Split(strings.Trim(specPath, "/"), "/")
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(specParts) != len(requestParts) {
		return nil
	}

	params := make(map[string]string)

	for i, specPart := range specParts {
		if strings.HasPrefix(specPart, "{") && strings.HasSuffix(specPart, "}") {
			// Parameter part
			paramName := strings.Trim(specPart, "{}")
			params[paramName] = requestParts[i]
		} else if specPart != requestParts[i] {
			// Literal part that doesn't match
			return nil
		}
	}

	return params
}

// determineResponseCode determines the appropriate response code to mock
func determineResponseCode(operation *openapi.Operation) string {
	if operation == nil || len(operation.Responses) == 0 {
		return "200"
	}

	// Prioritize successful responses
	successCodes := []string{"200", "201", "202", "204"}
	for _, code := range successCodes {
		if _, exists := operation.Responses[code]; exists {
			return code
		}
	}

	// Look for any 2xx response
	for code := range operation.Responses {
		if len(code) == 3 && code[0] == '2' {
			return code
		}
	}

	// Fall back to the first available response
	for code := range operation.Responses {
		return code
	}

	return "200"
}

// getResponseSchema extracts the response schema for a given status code
func getResponseSchema(operation *openapi.Operation, statusCode string) (*openapi.Schema, string) {
	if operation == nil || operation.Responses == nil {
		return nil, ""
	}

	response, exists := operation.Responses[statusCode]
	if !exists {
		return nil, ""
	}

	// Look for JSON content first
	if mediaObj, exists := response.Content["application/json"]; exists && mediaObj.Schema != nil {
		return mediaObj.Schema, "application/json"
	}

	// Fall back to any available content type
	for mediaType, mediaObj := range response.Content {
		if mediaObj.Schema != nil {
			return mediaObj.Schema, mediaType
		}
	}

	return nil, ""
}

// getResponseSchemaWithMediaType extracts the response schema and MediaTypeObject for a given status code
func getResponseSchemaWithMediaType(operation *openapi.Operation, statusCode string) (*openapi.Schema, string, *openapi.MediaTypeObject) {
	if operation == nil || operation.Responses == nil {
		return nil, "", nil
	}

	response, exists := operation.Responses[statusCode]
	if !exists {
		return nil, "", nil
	}

	// Look for JSON content first
	if mediaObj, exists := response.Content["application/json"]; exists && mediaObj.Schema != nil {
		return mediaObj.Schema, "application/json", &mediaObj
	}

	// Fall back to any available content type
	for mediaType, mediaObj := range response.Content {
		if mediaObj.Schema != nil {
			return mediaObj.Schema, mediaType, &mediaObj
		}
	}

	return nil, "", nil
}

// setResponseHeaders sets appropriate response headers
func setResponseHeaders(ctx *fasthttp.RequestCtx, statusCode, mediaType string) {
	// Set status code
	if code, err := strconv.Atoi(statusCode); err == nil {
		ctx.SetStatusCode(code)
	} else {
		ctx.SetStatusCode(fasthttp.StatusOK)
	}

	// Set content type
	if mediaType != "" {
		ctx.Response.Header.Set("Content-Type", mediaType)
	} else {
		ctx.Response.Header.Set("Content-Type", "application/json")
	}

	// Add mock-specific headers
	ctx.Response.Header.Set("X-Mock-Response", "true")
	ctx.Response.Header.Set("X-Mock-Generator", "vanta")

	// Add CORS headers for browser compatibility
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
}

// sendMockResponse serializes and sends the mock response
func sendMockResponse(ctx *fasthttp.RequestCtx, mockData interface{}, logger *zap.Logger) error {
	if mockData == nil {
		ctx.SetBody([]byte("{}"))
		return nil
	}

	responseBytes, err := json.Marshal(mockData)
	if err != nil {
		logger.Error("Failed to marshal mock response", zap.Error(err))
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	ctx.SetBody(responseBytes)

	logger.Debug("Mock response sent",
		zap.Int("response_size", len(responseBytes)),
		zap.Int("status_code", ctx.Response.StatusCode()),
	)

	return nil
}

// Error handlers

// handleEndpointNotFound handles requests for unknown endpoints
func handleEndpointNotFound(ctx *fasthttp.RequestCtx, method, path string, logger *zap.Logger) error {
	ctx.SetStatusCode(fasthttp.StatusNotFound)
	ctx.SetContentType("application/json")

	errorResponse := map[string]interface{}{
		"error":   "Endpoint not found",
		"message": fmt.Sprintf("No mock endpoint found for %s %s", method, path),
		"method":  method,
		"path":    path,
	}

	responseBytes, _ := json.Marshal(errorResponse)
	ctx.SetBody(responseBytes)

	logger.Warn("Endpoint not found",
		zap.String("method", method),
		zap.String("path", path),
	)

	return nil
}

// handleNoResponseSchema handles cases where no response schema is found
func handleNoResponseSchema(ctx *fasthttp.RequestCtx, statusCode string, logger *zap.Logger) error {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")

	response := map[string]interface{}{
		"message":     "Mock response (no schema defined)",
		"status_code": statusCode,
	}

	responseBytes, _ := json.Marshal(response)
	ctx.SetBody(responseBytes)

	logger.Debug("No response schema found, returning default response",
		zap.String("status_code", statusCode),
	)

	return nil
}

// handleGenerationError handles errors during mock data generation
func handleGenerationError(ctx *fasthttp.RequestCtx, err error, logger *zap.Logger) error {
	ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	ctx.SetContentType("application/json")

	errorResponse := map[string]interface{}{
		"error":   "Mock generation failed",
		"message": "Failed to generate mock data",
		"details": err.Error(),
	}

	responseBytes, _ := json.Marshal(errorResponse)
	ctx.SetBody(responseBytes)

	logger.Error("Mock generation failed", zap.Error(err))

	return nil
}

// OptionsHandler handles CORS preflight requests
func OptionsHandler() HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
		ctx.SetStatusCode(fasthttp.StatusNoContent)
		return nil
	}
}

// HealthCheckHandler provides a simple health check endpoint
func HealthCheckHandler(spec *openapi.Specification) HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)

		health := map[string]interface{}{
			"status":    "healthy",
			"service":   "vanta-mocker",
			"endpoints": len(spec.Paths),
			"timestamp": ctx.Time().Unix(),
		}

		responseBytes, _ := json.Marshal(health)
		ctx.SetBody(responseBytes)

		return nil
	}
}

// InfoHandler provides information about the loaded OpenAPI specification
func InfoHandler(spec *openapi.Specification) HandlerFunc {
	return func(ctx *fasthttp.RequestCtx) error {
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)

		// Count endpoints by method
		methodCounts := make(map[string]int)
		totalEndpoints := 0

		for _, pathItem := range spec.Paths {
			if pathItem.GET != nil {
				methodCounts["GET"]++
				totalEndpoints++
			}
			if pathItem.POST != nil {
				methodCounts["POST"]++
				totalEndpoints++
			}
			if pathItem.PUT != nil {
				methodCounts["PUT"]++
				totalEndpoints++
			}
			if pathItem.DELETE != nil {
				methodCounts["DELETE"]++
				totalEndpoints++
			}
			if pathItem.PATCH != nil {
				methodCounts["PATCH"]++
				totalEndpoints++
			}
		}

		info := map[string]interface{}{
			"title":               spec.Info.Title,
			"version":             spec.Info.Version,
			"description":         spec.Info.Description,
			"openapi_version":     spec.Version,
			"total_paths":         len(spec.Paths),
			"total_endpoints":     totalEndpoints,
			"endpoints_by_method": methodCounts,
			"schemas":             len(spec.Schemas),
		}

		responseBytes, _ := json.Marshal(info)
		ctx.SetBody(responseBytes)

		return nil
	}
}

// Add a method to get the seed from DefaultDataGenerator (we'll need to add this to the generator)
// This is a placeholder for now - we'll need to implement this in the generator
type SeedProvider interface {
	GetSeed() int64
}

// Ensure DefaultDataGenerator implements SeedProvider
var _ SeedProvider = (*openapi.DefaultDataGenerator)(nil)
