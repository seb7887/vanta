package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vanta/pkg/openapi"
)

type RequestValidator struct {
	spec     *openapi.Specification
	config   *Config
	reporter *Reporter
}

type ValidationResult struct {
	Valid      bool                 `json:"valid"`
	Errors     []ValidationError    `json:"errors,omitempty"`
	Warnings   []ValidationWarning  `json:"warnings,omitempty"`
	Endpoint   *EndpointInfo        `json:"endpoint,omitempty"`
	Timestamp  time.Time           `json:"timestamp"`
	RequestID  string              `json:"request_id,omitempty"`
}

type ValidationError struct {
	Type        string      `json:"type"`
	Field       string      `json:"field"`
	Location    string      `json:"location"` // query, header, path, body
	Message     string      `json:"message"`
	Expected    interface{} `json:"expected,omitempty"`
	Actual      interface{} `json:"actual,omitempty"`
	Severity    string      `json:"severity"` // error, warning
}

type ValidationWarning struct {
	Type     string      `json:"type"`
	Field    string      `json:"field"`
	Location string      `json:"location"`
	Message  string      `json:"message"`
	Value    interface{} `json:"value,omitempty"`
}

type EndpointInfo struct {
	Path         string `json:"path"`
	Method       string `json:"method"`
	OperationID  string `json:"operation_id,omitempty"`
	Found        bool   `json:"found"`
}

func NewRequestValidator(spec *openapi.Specification, config *Config) *RequestValidator {
	if config == nil {
		config = &Config{
			StrictMode:        false,
			ValidateHeaders:   true,
			ValidateQuery:     true,
			ValidatePath:      true,
			ValidateBody:      true,
			AllowExtraFields:  true,
		}
	}

	return &RequestValidator{
		spec:     spec,
		config:   config,
		reporter: NewReporter(),
	}
}

func (rv *RequestValidator) ValidateRequest(ctx context.Context, req *http.Request) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings:  []ValidationWarning{},
		Timestamp: time.Now(),
	}

	// Extract request ID from context if available
	if requestID, ok := ctx.Value("request_id").(string); ok {
		result.RequestID = requestID
	}

	// Find matching endpoint
	endpoint, operation := rv.findEndpoint(req.URL.Path, req.Method)
	if endpoint == nil {
		result.Endpoint = &EndpointInfo{
			Path:   req.URL.Path,
			Method: req.Method,
			Found:  false,
		}

		if rv.config.StrictMode {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "endpoint_not_found",
				Message:  fmt.Sprintf("Endpoint %s %s not found in specification", req.Method, req.URL.Path),
				Severity: "error",
			})
		} else {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Type:    "endpoint_not_found",
				Message: fmt.Sprintf("Endpoint %s %s not found in specification", req.Method, req.URL.Path),
			})
		}

		return result, nil
	}

	result.Endpoint = &EndpointInfo{
		Path:        endpoint.Path,
		Method:      endpoint.Method,
		OperationID: endpoint.OperationID,
		Found:       true,
	}

	// Validate path parameters
	if rv.config.ValidatePath {
		if errs, warns := rv.validatePathParameters(req, endpoint); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Validate query parameters
	if rv.config.ValidateQuery {
		if errs, warns := rv.validateQueryParameters(req, endpoint); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Validate headers
	if rv.config.ValidateHeaders {
		if errs, warns := rv.validateHeaders(req, endpoint); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Validate request body
	if rv.config.ValidateBody && operation.RequestBody != nil {
		if errs, warns := rv.validateRequestBody(req, operation); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Report to coverage tracker
	rv.reporter.RecordRequest(ctx, endpoint, result)

	return result, nil
}

func (rv *RequestValidator) findEndpoint(path, method string) (*openapi.Endpoint, *openapi.Operation) {
	// Direct path match first
	if pathItem, exists := rv.spec.Paths[path]; exists {
		if operation := rv.getOperationFromPathItem(&pathItem, method); operation != nil {
			return &openapi.Endpoint{
				Path:       path,
				Method:     method,
				Operation:  operation,
			}, operation
		}
	}

	// Pattern matching for path parameters
	for specPath, pathItem := range rv.spec.Paths {
		if rv.matchPath(specPath, path) {
			if operation := rv.getOperationFromPathItem(&pathItem, method); operation != nil {
				endpoint := &openapi.Endpoint{
					Path:       specPath,
					Method:     method,
					Operation:  operation,
				}

				// Extract parameters from path item and operation
				var allParams []openapi.Parameter
				if operation.Parameters != nil {
					allParams = append(allParams, operation.Parameters...)
				}

				endpoint.Parameters = allParams
				endpoint.OperationID = operation.OperationID

				return endpoint, operation
			}
		}
	}

	return nil, nil
}

func (rv *RequestValidator) getOperationFromPathItem(pathItem *openapi.PathItem, method string) *openapi.Operation {
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

func (rv *RequestValidator) matchPath(specPath, requestPath string) bool {
	// Convert OpenAPI path parameters to regex pattern
	pattern := regexp.QuoteMeta(specPath)
	pattern = regexp.MustCompile(`\\{([^}]+)\\}`).ReplaceAllString(pattern, `([^/]+)`)
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, requestPath)
	return matched
}

func (rv *RequestValidator) validatePathParameters(req *http.Request, endpoint *openapi.Endpoint) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Extract path parameters from the request path
	pathParams := rv.extractPathParameters(endpoint.Path, req.URL.Path)

	// Find path parameters in the operation
	pathParamSchemas := make(map[string]*openapi.Schema)
	for _, param := range endpoint.Parameters {
		if param.In == "path" {
			pathParamSchemas[param.Name] = param.Schema
		}
	}

	// Validate each required path parameter
	for paramName, schema := range pathParamSchemas {
		value, exists := pathParams[paramName]
		if !exists {
			errors = append(errors, ValidationError{
				Type:     "missing_parameter",
				Field:    paramName,
				Location: "path",
				Message:  fmt.Sprintf("Required path parameter '%s' is missing", paramName),
				Severity: "error",
			})
			continue
		}

		if schema != nil {
			if err := rv.validateValue(value, schema, paramName, "path"); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	return errors, warnings
}

func (rv *RequestValidator) validateQueryParameters(req *http.Request, endpoint *openapi.Endpoint) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	queryValues := req.URL.Query()

	// Find query parameters in the operation
	queryParamSchemas := make(map[string]*openapi.Parameter)
	for _, param := range endpoint.Parameters {
		if param.In == "query" {
			queryParamSchemas[param.Name] = &param
		}
	}

	// Validate required query parameters
	for paramName, param := range queryParamSchemas {
		values, exists := queryValues[paramName]

		if !exists && param.Required {
			errors = append(errors, ValidationError{
				Type:     "missing_parameter",
				Field:    paramName,
				Location: "query",
				Message:  fmt.Sprintf("Required query parameter '%s' is missing", paramName),
				Severity: "error",
			})
			continue
		}

		if exists && param.Schema != nil {
			for _, value := range values {
				if err := rv.validateValue(value, param.Schema, paramName, "query"); err != nil {
					errors = append(errors, *err)
				}
			}
		}
	}

	// Check for extra query parameters if not allowing extra fields
	if !rv.config.AllowExtraFields {
		for paramName := range queryValues {
			if _, exists := queryParamSchemas[paramName]; !exists {
				warnings = append(warnings, ValidationWarning{
					Type:     "extra_parameter",
					Field:    paramName,
					Location: "query",
					Message:  fmt.Sprintf("Unexpected query parameter '%s'", paramName),
					Value:    queryValues[paramName],
				})
			}
		}
	}

	return errors, warnings
}

func (rv *RequestValidator) validateHeaders(req *http.Request, endpoint *openapi.Endpoint) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Find header parameters in the operation
	headerParamSchemas := make(map[string]*openapi.Parameter)
	for _, param := range endpoint.Parameters {
		if param.In == "header" {
			headerParamSchemas[strings.ToLower(param.Name)] = &param
		}
	}

	// Validate required header parameters
	for paramName, param := range headerParamSchemas {
		value := req.Header.Get(paramName)

		if value == "" && param.Required {
			errors = append(errors, ValidationError{
				Type:     "missing_parameter",
				Field:    paramName,
				Location: "header",
				Message:  fmt.Sprintf("Required header parameter '%s' is missing", paramName),
				Severity: "error",
			})
			continue
		}

		if value != "" && param.Schema != nil {
			if err := rv.validateValue(value, param.Schema, paramName, "header"); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	return errors, warnings
}

func (rv *RequestValidator) validateRequestBody(req *http.Request, operation *openapi.Operation) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		if operation.RequestBody.Required {
			errors = append(errors, ValidationError{
				Type:     "missing_content_type",
				Location: "header",
				Message:  "Content-Type header is required for request body",
				Severity: "error",
			})
		}
		return errors, warnings
	}

	// Find matching media type
	var mediaType *openapi.MediaTypeObject
	for mt, obj := range operation.RequestBody.Content {
		if strings.HasPrefix(contentType, mt) {
			mediaType = &obj
			break
		}
	}

	if mediaType == nil {
		errors = append(errors, ValidationError{
			Type:     "unsupported_media_type",
			Location: "header",
			Message:  fmt.Sprintf("Content-Type '%s' is not supported by this endpoint", contentType),
			Expected: rv.getSupportedMediaTypes(operation.RequestBody.Content),
			Actual:   contentType,
			Severity: "error",
		})
		return errors, warnings
	}

	if mediaType.Schema == nil {
		return errors, warnings
	}

	// Validate JSON body
	if strings.Contains(contentType, "application/json") {
		if errs, warns := rv.validateJSONBody(req, mediaType.Schema); len(errs) > 0 || len(warns) > 0 {
			errors = append(errors, errs...)
			warnings = append(warnings, warns...)
		}
	}

	return errors, warnings
}

func (rv *RequestValidator) validateJSONBody(req *http.Request, schema *openapi.Schema) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Parse JSON body
	var bodyData interface{}
	if err := json.NewDecoder(req.Body).Decode(&bodyData); err != nil {
		errors = append(errors, ValidationError{
			Type:     "invalid_json",
			Location: "body",
			Message:  fmt.Sprintf("Invalid JSON in request body: %v", err),
			Severity: "error",
		})
		return errors, warnings
	}

	// Validate against schema
	if errs := rv.validateValueAgainstSchema(bodyData, schema, "body", "body"); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors, warnings
}

func (rv *RequestValidator) validateValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	switch schema.Type {
	case "string":
		return rv.validateStringValue(value, schema, fieldName, location)
	case "integer":
		return rv.validateIntegerValue(value, schema, fieldName, location)
	case "number":
		return rv.validateNumberValue(value, schema, fieldName, location)
	case "boolean":
		return rv.validateBooleanValue(value, schema, fieldName, location)
	default:
		return nil
	}
}

func (rv *RequestValidator) validateStringValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	// Check enum
	if len(schema.Enum) > 0 {
		valid := false
		for _, enumValue := range schema.Enum {
			if str, ok := enumValue.(string); ok && str == value {
				valid = true
				break
			}
		}
		if !valid {
			return &ValidationError{
				Type:     "invalid_enum_value",
				Field:    fieldName,
				Location: location,
				Message:  fmt.Sprintf("Value '%s' is not in allowed enum values", value),
				Expected: schema.Enum,
				Actual:   value,
				Severity: "error",
			}
		}
	}

	// Check pattern
	if schema.Pattern != "" {
		if matched, _ := regexp.MatchString(schema.Pattern, value); !matched {
			return &ValidationError{
				Type:     "invalid_pattern",
				Field:    fieldName,
				Location: location,
				Message:  fmt.Sprintf("Value '%s' does not match required pattern", value),
				Expected: schema.Pattern,
				Actual:   value,
				Severity: "error",
			}
		}
	}

	// Check length constraints
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		return &ValidationError{
			Type:     "string_too_short",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("String length %d is less than minimum %d", len(value), *schema.MinLength),
			Expected: *schema.MinLength,
			Actual:   len(value),
			Severity: "error",
		}
	}

	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		return &ValidationError{
			Type:     "string_too_long",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("String length %d exceeds maximum %d", len(value), *schema.MaxLength),
			Expected: *schema.MaxLength,
			Actual:   len(value),
			Severity: "error",
		}
	}

	return nil
}

func (rv *RequestValidator) validateIntegerValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return &ValidationError{
			Type:     "invalid_integer",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value '%s' is not a valid integer", value),
			Expected: "integer",
			Actual:   value,
			Severity: "error",
		}
	}

	// Check range constraints
	if schema.Minimum != nil && float64(intValue) < *schema.Minimum {
		return &ValidationError{
			Type:     "value_too_small",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value %d is less than minimum %.0f", intValue, *schema.Minimum),
			Expected: *schema.Minimum,
			Actual:   intValue,
			Severity: "error",
		}
	}

	if schema.Maximum != nil && float64(intValue) > *schema.Maximum {
		return &ValidationError{
			Type:     "value_too_large",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value %d exceeds maximum %.0f", intValue, *schema.Maximum),
			Expected: *schema.Maximum,
			Actual:   intValue,
			Severity: "error",
		}
	}

	return nil
}

func (rv *RequestValidator) validateNumberValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return &ValidationError{
			Type:     "invalid_number",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value '%s' is not a valid number", value),
			Expected: "number",
			Actual:   value,
			Severity: "error",
		}
	}

	// Check range constraints
	if schema.Minimum != nil && floatValue < *schema.Minimum {
		return &ValidationError{
			Type:     "value_too_small",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value %f is less than minimum %f", floatValue, *schema.Minimum),
			Expected: *schema.Minimum,
			Actual:   floatValue,
			Severity: "error",
		}
	}

	if schema.Maximum != nil && floatValue > *schema.Maximum {
		return &ValidationError{
			Type:     "value_too_large",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value %f exceeds maximum %f", floatValue, *schema.Maximum),
			Expected: *schema.Maximum,
			Actual:   floatValue,
			Severity: "error",
		}
	}

	return nil
}

func (rv *RequestValidator) validateBooleanValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	_, err := strconv.ParseBool(value)
	if err != nil {
		return &ValidationError{
			Type:     "invalid_boolean",
			Field:    fieldName,
			Location: location,
			Message:  fmt.Sprintf("Value '%s' is not a valid boolean", value),
			Expected: "boolean",
			Actual:   value,
			Severity: "error",
		}
	}

	return nil
}

func (rv *RequestValidator) validateValueAgainstSchema(value interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
	var errors []ValidationError

	// This is a simplified schema validation - in production you'd want to use a full JSON Schema validator
	switch schema.Type {
	case "object":
		if obj, ok := value.(map[string]interface{}); ok {
			errors = append(errors, rv.validateObjectAgainstSchema(obj, schema, fieldPath, location)...)
		} else {
			errors = append(errors, ValidationError{
				Type:     "invalid_type",
				Field:    fieldPath,
				Location: location,
				Message:  "Expected object type",
				Expected: "object",
				Actual:   fmt.Sprintf("%T", value),
				Severity: "error",
			})
		}
	case "array":
		if arr, ok := value.([]interface{}); ok {
			errors = append(errors, rv.validateArrayAgainstSchema(arr, schema, fieldPath, location)...)
		} else {
			errors = append(errors, ValidationError{
				Type:     "invalid_type",
				Field:    fieldPath,
				Location: location,
				Message:  "Expected array type",
				Expected: "array",
				Actual:   fmt.Sprintf("%T", value),
				Severity: "error",
			})
		}
	}

	return errors
}

func (rv *RequestValidator) validateObjectAgainstSchema(obj map[string]interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
	var errors []ValidationError

	// Check required fields
	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}

	for field := range requiredFields {
		if _, exists := obj[field]; !exists {
			errors = append(errors, ValidationError{
				Type:     "missing_required_field",
				Field:    fmt.Sprintf("%s.%s", fieldPath, field),
				Location: location,
				Message:  fmt.Sprintf("Required field '%s' is missing", field),
				Severity: "error",
			})
		}
	}

	// Validate properties
	if schema.Properties != nil {
		for field, value := range obj {
			fieldSchema, exists := schema.Properties[field]
			if !exists && !rv.config.AllowExtraFields {
				continue // Skip extra fields if allowed
			}

			if exists {
				fieldErrors := rv.validateValueAgainstSchema(value, fieldSchema, fmt.Sprintf("%s.%s", fieldPath, field), location)
				errors = append(errors, fieldErrors...)
			}
		}
	}

	return errors
}

func (rv *RequestValidator) validateArrayAgainstSchema(arr []interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
	var errors []ValidationError

	// Check array length constraints
	if schema.MinItems != nil && len(arr) < *schema.MinItems {
		errors = append(errors, ValidationError{
			Type:     "array_too_short",
			Field:    fieldPath,
			Location: location,
			Message:  fmt.Sprintf("Array length %d is less than minimum %d", len(arr), *schema.MinItems),
			Expected: *schema.MinItems,
			Actual:   len(arr),
			Severity: "error",
		})
	}

	if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
		errors = append(errors, ValidationError{
			Type:     "array_too_long",
			Field:    fieldPath,
			Location: location,
			Message:  fmt.Sprintf("Array length %d exceeds maximum %d", len(arr), *schema.MaxItems),
			Expected: *schema.MaxItems,
			Actual:   len(arr),
			Severity: "error",
		})
	}

	// Validate items
	if schema.Items != nil {
		for i, item := range arr {
			itemErrors := rv.validateValueAgainstSchema(item, schema.Items, fmt.Sprintf("%s[%d]", fieldPath, i), location)
			errors = append(errors, itemErrors...)
		}
	}

	return errors
}

func (rv *RequestValidator) extractPathParameters(specPath, requestPath string) map[string]string {
	params := make(map[string]string)

	// Convert spec path to regex and extract parameter names
	paramNames := regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(specPath, -1)
	if len(paramNames) == 0 {
		return params
	}

	// Create regex pattern to extract values
	pattern := regexp.QuoteMeta(specPath)
	pattern = regexp.MustCompile(`\\{([^}]+)\\}`).ReplaceAllString(pattern, `([^/]+)`)

	re := regexp.MustCompile("^" + pattern + "$")
	matches := re.FindStringSubmatch(requestPath)

	if len(matches) > 1 {
		for i, paramMatch := range paramNames {
			if i+1 < len(matches) {
				params[paramMatch[1]] = matches[i+1]
			}
		}
	}

	return params
}

func (rv *RequestValidator) getSupportedMediaTypes(content map[string]openapi.MediaTypeObject) []string {
	types := make([]string, 0, len(content))
	for mediaType := range content {
		types = append(types, mediaType)
	}
	return types
}