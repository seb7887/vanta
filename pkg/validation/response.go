package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/seb7887/vanta/pkg/openapi"
)

type ResponseValidator struct {
	spec     *openapi.Specification
	config   *Config
	reporter *Reporter
}

type ResponseValidationResult struct {
	Valid       bool                 `json:"valid"`
	StatusCode  int                  `json:"status_code"`
	Errors      []ValidationError    `json:"errors,omitempty"`
	Warnings    []ValidationWarning  `json:"warnings,omitempty"`
	Endpoint    *EndpointInfo        `json:"endpoint,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
	RequestID   string              `json:"request_id,omitempty"`
	ContentType string              `json:"content_type,omitempty"`
}

type MockResponse struct {
	StatusCode  int                 `json:"status_code"`
	Headers     map[string]string   `json:"headers"`
	Body        interface{}         `json:"body"`
	ContentType string              `json:"content_type"`
}

func NewResponseValidator(spec *openapi.Specification, config *Config) *ResponseValidator {
	if config == nil {
		config = &Config{
			StrictMode:           false,
			ValidateHeaders:      true,
			ValidateStatusCodes:  true,
			ValidateBody:         true,
			AllowExtraFields:     true,
		}
	}

	return &ResponseValidator{
		spec:     spec,
		config:   config,
		reporter: NewReporter(),
	}
}

func (rv *ResponseValidator) ValidateResponse(ctx context.Context, req *http.Request, resp *http.Response) (*ResponseValidationResult, error) {
	result := &ResponseValidationResult{
		Valid:       true,
		StatusCode:  resp.StatusCode,
		Errors:      []ValidationError{},
		Warnings:    []ValidationWarning{},
		Timestamp:   time.Now(),
		ContentType: resp.Header.Get("Content-Type"),
	}

	// Extract request ID from context if available
	if requestID, ok := ctx.Value("request_id").(string); ok {
		result.RequestID = requestID
	}

	// Find matching endpoint
	endpoint, operation := rv.findEndpoint(req.URL.Path, req.Method)
	if endpoint == nil {
		if rv.config.StrictMode {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "endpoint_not_found",
				Message:  fmt.Sprintf("Endpoint %s %s not found in specification", req.Method, req.URL.Path),
				Severity: "error",
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

	// Validate status code
	if rv.config.ValidateStatusCodes {
		if errs, warns := rv.validateStatusCode(resp.StatusCode, operation); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Get response definition for this status code
	responseSpec := rv.getResponseSpec(resp.StatusCode, operation)
	if responseSpec == nil {
		// Try default response
		if defaultResp, exists := operation.Responses["default"]; exists {
			responseSpec = &defaultResp
		}
	}

	if responseSpec != nil {
		// Validate response headers
		if rv.config.ValidateHeaders {
			if errs, warns := rv.validateResponseHeaders(resp, responseSpec); len(errs) > 0 || len(warns) > 0 {
				result.Errors = append(result.Errors, errs...)
				result.Warnings = append(result.Warnings, warns...)
				if len(errs) > 0 {
					result.Valid = false
				}
			}
		}

		// Validate response body
		if rv.config.ValidateBody {
			if errs, warns := rv.validateResponseBody(resp, responseSpec); len(errs) > 0 || len(warns) > 0 {
				result.Errors = append(result.Errors, errs...)
				result.Warnings = append(result.Warnings, warns...)
				if len(errs) > 0 {
					result.Valid = false
				}
			}
		}
	}

	// Report to coverage tracker
	rv.reporter.RecordResponse(ctx, endpoint, result)

	return result, nil
}

func (rv *ResponseValidator) ValidateMockResponse(ctx context.Context, req *http.Request, mockResp *MockResponse) (*ResponseValidationResult, error) {
	result := &ResponseValidationResult{
		Valid:       true,
		StatusCode:  mockResp.StatusCode,
		Errors:      []ValidationError{},
		Warnings:    []ValidationWarning{},
		Timestamp:   time.Now(),
		ContentType: mockResp.ContentType,
	}

	// Extract request ID from context if available
	if requestID, ok := ctx.Value("request_id").(string); ok {
		result.RequestID = requestID
	}

	// Find matching endpoint
	endpoint, operation := rv.findEndpoint(req.URL.Path, req.Method)
	if endpoint == nil {
		if rv.config.StrictMode {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "endpoint_not_found",
				Message:  fmt.Sprintf("Endpoint %s %s not found in specification", req.Method, req.URL.Path),
				Severity: "error",
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

	// Validate status code
	if rv.config.ValidateStatusCodes {
		if errs, warns := rv.validateStatusCode(mockResp.StatusCode, operation); len(errs) > 0 || len(warns) > 0 {
			result.Errors = append(result.Errors, errs...)
			result.Warnings = append(result.Warnings, warns...)
			if len(errs) > 0 {
				result.Valid = false
			}
		}
	}

	// Get response definition for this status code
	responseSpec := rv.getResponseSpec(mockResp.StatusCode, operation)
	if responseSpec == nil {
		// Try default response
		if defaultResp, exists := operation.Responses["default"]; exists {
			responseSpec = &defaultResp
		}
	}

	if responseSpec != nil {
		// Validate mock response headers
		if rv.config.ValidateHeaders {
			if errs, warns := rv.validateMockResponseHeaders(mockResp.Headers, responseSpec); len(errs) > 0 || len(warns) > 0 {
				result.Errors = append(result.Errors, errs...)
				result.Warnings = append(result.Warnings, warns...)
				if len(errs) > 0 {
					result.Valid = false
				}
			}
		}

		// Validate mock response body
		if rv.config.ValidateBody {
			if errs, warns := rv.validateMockResponseBody(mockResp, responseSpec); len(errs) > 0 || len(warns) > 0 {
				result.Errors = append(result.Errors, errs...)
				result.Warnings = append(result.Warnings, warns...)
				if len(errs) > 0 {
					result.Valid = false
				}
			}
		}
	}

	return result, nil
}

func (rv *ResponseValidator) findEndpoint(path, method string) (*openapi.Endpoint, *openapi.Operation) {
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

				endpoint.OperationID = operation.OperationID
				return endpoint, operation
			}
		}
	}

	return nil, nil
}

func (rv *ResponseValidator) getOperationFromPathItem(pathItem *openapi.PathItem, method string) *openapi.Operation {
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

func (rv *ResponseValidator) matchPath(specPath, requestPath string) bool {
	// Simple path matching - in production you'd want more sophisticated matching
	if specPath == requestPath {
		return true
	}

	// Handle path parameters
	specParts := strings.Split(specPath, "/")
	requestParts := strings.Split(requestPath, "/")

	if len(specParts) != len(requestParts) {
		return false
	}

	for i, specPart := range specParts {
		if strings.HasPrefix(specPart, "{") && strings.HasSuffix(specPart, "}") {
			// This is a path parameter, skip validation
			continue
		}
		if specPart != requestParts[i] {
			return false
		}
	}

	return true
}

func (rv *ResponseValidator) validateStatusCode(statusCode int, operation *openapi.Operation) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Check if status code is defined in the operation
	statusStr := strconv.Itoa(statusCode)

	if _, exists := operation.Responses[statusStr]; exists {
		return errors, warnings
	}

	// Check for wildcard patterns (2XX, 4XX, etc.)
	statusClass := fmt.Sprintf("%dXX", statusCode/100)
	if _, exists := operation.Responses[statusClass]; exists {
		return errors, warnings
	}

	// Check for default response
	if _, exists := operation.Responses["default"]; exists {
		return errors, warnings
	}

	// Status code not found in specification
	if rv.config.StrictMode {
		errors = append(errors, ValidationError{
			Type:     "undefined_status_code",
			Message:  fmt.Sprintf("Status code %d is not defined in the OpenAPI specification", statusCode),
			Expected: rv.getDefinedStatusCodes(operation.Responses),
			Actual:   statusCode,
			Severity: "error",
		})
	} else {
		warnings = append(warnings, ValidationWarning{
			Type:    "undefined_status_code",
			Message: fmt.Sprintf("Status code %d is not defined in the OpenAPI specification", statusCode),
			Value:   statusCode,
		})
	}

	return errors, warnings
}

func (rv *ResponseValidator) validateResponseHeaders(resp *http.Response, responseSpec *openapi.Response) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if responseSpec.Headers == nil {
		return errors, warnings
	}

	// Validate required headers
	for headerName, headerSpec := range responseSpec.Headers {
		value := resp.Header.Get(headerName)

		// For now, we'll just check if required headers are present
		// In a full implementation, you'd validate against the header schema
		if value == "" {
			// Assuming headers are required if defined (OpenAPI doesn't have a required field for response headers)
			if rv.config.StrictMode {
				errors = append(errors, ValidationError{
					Type:     "missing_response_header",
					Field:    headerName,
					Location: "header",
					Message:  fmt.Sprintf("Expected response header '%s' is missing", headerName),
					Severity: "error",
				})
			} else {
				warnings = append(warnings, ValidationWarning{
					Type:     "missing_response_header",
					Field:    headerName,
					Location: "header",
					Message:  fmt.Sprintf("Expected response header '%s' is missing", headerName),
				})
			}
		} else if headerSpec.Schema != nil {
			// Validate header value against schema
			if err := rv.validateHeaderValue(value, headerSpec.Schema, headerName); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	return errors, warnings
}

func (rv *ResponseValidator) validateMockResponseHeaders(headers map[string]string, responseSpec *openapi.Response) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if responseSpec.Headers == nil {
		return errors, warnings
	}

	// Validate required headers
	for headerName, headerSpec := range responseSpec.Headers {
		value, exists := headers[headerName]

		if !exists {
			if rv.config.StrictMode {
				errors = append(errors, ValidationError{
					Type:     "missing_response_header",
					Field:    headerName,
					Location: "header",
					Message:  fmt.Sprintf("Expected response header '%s' is missing", headerName),
					Severity: "error",
				})
			} else {
				warnings = append(warnings, ValidationWarning{
					Type:     "missing_response_header",
					Field:    headerName,
					Location: "header",
					Message:  fmt.Sprintf("Expected response header '%s' is missing", headerName),
				})
			}
		} else if headerSpec.Schema != nil {
			// Validate header value against schema
			if err := rv.validateHeaderValue(value, headerSpec.Schema, headerName); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	return errors, warnings
}

func (rv *ResponseValidator) validateResponseBody(resp *http.Response, responseSpec *openapi.Response) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if responseSpec.Content == nil || len(responseSpec.Content) == 0 {
		return errors, warnings
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		if rv.config.StrictMode {
			errors = append(errors, ValidationError{
				Type:     "missing_content_type",
				Location: "header",
				Message:  "Response Content-Type header is missing",
				Severity: "error",
			})
		}
		return errors, warnings
	}

	// Find matching media type
	var mediaType *openapi.MediaTypeObject
	for mt, obj := range responseSpec.Content {
		if strings.HasPrefix(contentType, mt) {
			mediaType = &obj
			break
		}
	}

	if mediaType == nil {
		if rv.config.StrictMode {
			errors = append(errors, ValidationError{
				Type:     "unsupported_response_media_type",
				Location: "header",
				Message:  fmt.Sprintf("Response Content-Type '%s' is not defined in specification", contentType),
				Expected: rv.getSupportedMediaTypes(responseSpec.Content),
				Actual:   contentType,
				Severity: "error",
			})
		}
		return errors, warnings
	}

	if mediaType.Schema == nil {
		return errors, warnings
	}

	// Validate JSON response body
	if strings.Contains(contentType, "application/json") {
		if errs, warns := rv.validateJSONResponseBody(resp, mediaType.Schema); len(errs) > 0 || len(warns) > 0 {
			errors = append(errors, errs...)
			warnings = append(warnings, warns...)
		}
	}

	return errors, warnings
}

func (rv *ResponseValidator) validateMockResponseBody(mockResp *MockResponse, responseSpec *openapi.Response) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if responseSpec.Content == nil || len(responseSpec.Content) == 0 {
		return errors, warnings
	}

	contentType := mockResp.ContentType
	if contentType == "" {
		if rv.config.StrictMode {
			errors = append(errors, ValidationError{
				Type:     "missing_content_type",
				Location: "header",
				Message:  "Response Content-Type is missing",
				Severity: "error",
			})
		}
		return errors, warnings
	}

	// Find matching media type
	var mediaType *openapi.MediaTypeObject
	for mt, obj := range responseSpec.Content {
		if strings.HasPrefix(contentType, mt) {
			mediaType = &obj
			break
		}
	}

	if mediaType == nil {
		if rv.config.StrictMode {
			errors = append(errors, ValidationError{
				Type:     "unsupported_response_media_type",
				Location: "header",
				Message:  fmt.Sprintf("Response Content-Type '%s' is not defined in specification", contentType),
				Expected: rv.getSupportedMediaTypes(responseSpec.Content),
				Actual:   contentType,
				Severity: "error",
			})
		}
		return errors, warnings
	}

	if mediaType.Schema == nil {
		return errors, warnings
	}

	// Validate mock response body against schema
	if errs := rv.validateValueAgainstSchema(mockResp.Body, mediaType.Schema, "body", "body"); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	return errors, warnings
}

func (rv *ResponseValidator) validateJSONResponseBody(resp *http.Response, schema *openapi.Schema) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errors = append(errors, ValidationError{
			Type:     "body_read_error",
			Location: "body",
			Message:  fmt.Sprintf("Failed to read response body: %v", err),
			Severity: "error",
		})
		return errors, warnings
	}

	// Restore the body for later use
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))

	// Parse JSON body
	var bodyData interface{}
	if err := json.Unmarshal(body, &bodyData); err != nil {
		errors = append(errors, ValidationError{
			Type:     "invalid_json",
			Location: "body",
			Message:  fmt.Sprintf("Invalid JSON in response body: %v", err),
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

func (rv *ResponseValidator) validateHeaderValue(value string, schema *openapi.Schema, headerName string) *ValidationError {
	switch schema.Type {
	case "string":
		return rv.validateStringValue(value, schema, headerName, "header")
	case "integer":
		return rv.validateIntegerValue(value, schema, headerName, "header")
	case "number":
		return rv.validateNumberValue(value, schema, headerName, "header")
	case "boolean":
		return rv.validateBooleanValue(value, schema, headerName, "header")
	default:
		return nil
	}
}

func (rv *ResponseValidator) validateStringValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	// Same validation logic as in request validator
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

	return nil
}

func (rv *ResponseValidator) validateIntegerValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	// Same validation logic as in request validator
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

	return nil
}

func (rv *ResponseValidator) validateNumberValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
	// Same validation logic as in request validator
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

	return nil
}

func (rv *ResponseValidator) validateBooleanValue(value string, schema *openapi.Schema, fieldName, location string) *ValidationError {
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

func (rv *ResponseValidator) validateValueAgainstSchema(value interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
	var errors []ValidationError

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

func (rv *ResponseValidator) validateObjectAgainstSchema(obj map[string]interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
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
				continue
			}

			if exists {
				fieldErrors := rv.validateValueAgainstSchema(value, fieldSchema, fmt.Sprintf("%s.%s", fieldPath, field), location)
				errors = append(errors, fieldErrors...)
			}
		}
	}

	return errors
}

func (rv *ResponseValidator) validateArrayAgainstSchema(arr []interface{}, schema *openapi.Schema, fieldPath, location string) []ValidationError {
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

func (rv *ResponseValidator) getResponseSpec(statusCode int, operation *openapi.Operation) *openapi.Response {
	// Try exact status code match
	statusStr := strconv.Itoa(statusCode)
	if resp, exists := operation.Responses[statusStr]; exists {
		return &resp
	}

	// Try wildcard pattern (2XX, 4XX, etc.)
	statusClass := fmt.Sprintf("%dXX", statusCode/100)
	if resp, exists := operation.Responses[statusClass]; exists {
		return &resp
	}

	return nil
}

func (rv *ResponseValidator) getDefinedStatusCodes(responses map[string]openapi.Response) []string {
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	return codes
}

func (rv *ResponseValidator) getSupportedMediaTypes(content map[string]openapi.MediaTypeObject) []string {
	types := make([]string, 0, len(content))
	for mediaType := range content {
		types = append(types, mediaType)
	}
	return types
}
