package chaos

import (
	"fmt"
	"math/rand"
	"reflect"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// ErrorInjector implements chaos injection by returning HTTP error responses
type ErrorInjector struct {
	logger *zap.Logger
	rng    *rand.Rand
}

// NewErrorInjector creates a new error injector
func NewErrorInjector(logger *zap.Logger, rng *rand.Rand) *ErrorInjector {
	return &ErrorInjector{
		logger: logger,
		rng:    rng,
	}
}

// Type returns the type of chaos this injector handles
func (e *ErrorInjector) Type() string {
	return "error"
}

// Validate validates the parameters for error injection
func (e *ErrorInjector) Validate(params map[string]interface{}) error {
	errorCodesRaw, hasErrorCodes := params["error_codes"]
	if !hasErrorCodes {
		return fmt.Errorf("error injector requires 'error_codes' parameter")
	}
	
	// Validate error_codes is a slice
	errorCodesValue := reflect.ValueOf(errorCodesRaw)
	if errorCodesValue.Kind() != reflect.Slice {
		return fmt.Errorf("error_codes must be an array of HTTP status codes")
	}
	
	// Validate each error code
	for i := 0; i < errorCodesValue.Len(); i++ {
		codeValue := errorCodesValue.Index(i)
		
		var code int
		switch codeValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			code = int(codeValue.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			code = int(codeValue.Uint())
		case reflect.Float32, reflect.Float64:
			code = int(codeValue.Float())
		default:
			return fmt.Errorf("error_codes must contain only numeric values")
		}
		
		if code < 400 || code > 599 {
			return fmt.Errorf("error code %d must be between 400 and 599", code)
		}
	}
	
	// Validate custom_body if present
	if customBodyRaw, hasCustomBody := params["custom_body"]; hasCustomBody {
		if _, ok := customBodyRaw.(string); !ok {
			return fmt.Errorf("custom_body must be a string")
		}
	}
	
	return nil
}

// Inject applies error chaos to the request context
func (e *ErrorInjector) Inject(ctx *fasthttp.RequestCtx, params map[string]interface{}) error {
	errorCodesRaw := params["error_codes"]
	errorCodes, err := e.parseErrorCodes(errorCodesRaw)
	if err != nil {
		return fmt.Errorf("failed to parse error codes: %w", err)
	}
	
	// Select random error code
	selectedCode := errorCodes[e.rng.Intn(len(errorCodes))]
	
	// Get custom body if provided
	customBody := e.getCustomBody(params, selectedCode)
	
	e.logger.Debug("Injecting error chaos",
		zap.String("path", string(ctx.Path())),
		zap.Int("status_code", selectedCode),
		zap.String("custom_body", customBody))
	
	// Set HTTP status code
	ctx.SetStatusCode(selectedCode)
	
	// Set Content-Type header
	ctx.SetContentType("application/json")
	
	// Set response body
	ctx.SetBodyString(customBody)
	
	return nil
}

// parseErrorCodes converts the interface{} error codes to []int
func (e *ErrorInjector) parseErrorCodes(errorCodesRaw interface{}) ([]int, error) {
	errorCodesValue := reflect.ValueOf(errorCodesRaw)
	if errorCodesValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("error_codes must be a slice")
	}
	
	codes := make([]int, errorCodesValue.Len())
	for i := 0; i < errorCodesValue.Len(); i++ {
		codeValue := errorCodesValue.Index(i)
		
		var code int
		switch codeValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			code = int(codeValue.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			code = int(codeValue.Uint())
		case reflect.Float32, reflect.Float64:
			code = int(codeValue.Float())
		default:
			return nil, fmt.Errorf("invalid error code type: %v", codeValue.Kind())
		}
		
		codes[i] = code
	}
	
	return codes, nil
}

// getCustomBody returns the custom body for the error response
func (e *ErrorInjector) getCustomBody(params map[string]interface{}, statusCode int) string {
	if customBodyRaw, hasCustomBody := params["custom_body"]; hasCustomBody {
		if customBody, ok := customBodyRaw.(string); ok && customBody != "" {
			return customBody
		}
	}
	
	// Return default error body based on status code
	return e.getDefaultErrorBody(statusCode)
}

// getDefaultErrorBody returns a default error message for the given status code
func (e *ErrorInjector) getDefaultErrorBody(statusCode int) string {
	errorMessages := map[int]string{
		400: `{"error": "Bad Request", "message": "The request could not be understood by the server due to malformed syntax."}`,
		401: `{"error": "Unauthorized", "message": "The request requires user authentication."}`,
		403: `{"error": "Forbidden", "message": "The server understood the request, but is refusing to fulfill it."}`,
		404: `{"error": "Not Found", "message": "The server has not found anything matching the Request-URI."}`,
		405: `{"error": "Method Not Allowed", "message": "The method specified in the Request-Line is not allowed for the resource."}`,
		408: `{"error": "Request Timeout", "message": "The client did not produce a request within the time that the server was prepared to wait."}`,
		409: `{"error": "Conflict", "message": "The request could not be completed due to a conflict with the current state of the resource."}`,
		429: `{"error": "Too Many Requests", "message": "The user has sent too many requests in a given amount of time."}`,
		500: `{"error": "Internal Server Error", "message": "The server encountered an unexpected condition which prevented it from fulfilling the request."}`,
		501: `{"error": "Not Implemented", "message": "The server does not support the functionality required to fulfill the request."}`,
		502: `{"error": "Bad Gateway", "message": "The server, while acting as a gateway or proxy, received an invalid response from the upstream server."}`,
		503: `{"error": "Service Unavailable", "message": "The server is currently unable to handle the request due to temporary overloading or maintenance."}`,
		504: `{"error": "Gateway Timeout", "message": "The server, while acting as a gateway or proxy, did not receive a timely response from the upstream server."}`,
		507: `{"error": "Insufficient Storage", "message": "The server is unable to store the representation needed to complete the request."}`,
	}
	
	if message, exists := errorMessages[statusCode]; exists {
		return message
	}
	
	// Fallback for unknown status codes
	return fmt.Sprintf(`{"error": "HTTP Error %d", "message": "An error occurred while processing the request."}`, statusCode)
}