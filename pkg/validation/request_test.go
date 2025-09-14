package validation

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"vanta/pkg/openapi"
)

func TestRequestValidator_ValidateRequest(t *testing.T) {
	// Create test OpenAPI specification
	spec := &openapi.Specification{
		Version: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: map[string]openapi.PathItem{
			"/users/{id}": {
				GET: &openapi.Operation{
					OperationID: "getUser",
					Parameters: []openapi.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema: &openapi.Schema{
								Type: "string",
							},
						},
						{
							Name:     "format",
							In:       "query",
							Required: false,
							Schema: &openapi.Schema{
								Type: "string",
								Enum: []interface{}{"json", "xml"},
							},
						},
					},
					Responses: map[string]openapi.Response{
						"200": {
							Description: "Success",
							Content: map[string]openapi.MediaTypeObject{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "object",
										Properties: map[string]*openapi.Schema{
											"id": {Type: "string"},
											"name": {Type: "string"},
										},
										Required: []string{"id", "name"},
									},
								},
							},
						},
					},
				},
			},
		},
		Schemas: make(map[string]*openapi.Schema),
	}

	config := &Config{
		Enabled:         true,
		StrictMode:      false,
		ValidateHeaders: true,
		ValidateQuery:   true,
		ValidatePath:    true,
		ValidateBody:    true,
	}

	validator := NewRequestValidator(spec, config)

	tests := []struct {
		name        string
		request     *http.Request
		expectValid bool
		expectError bool
	}{
		{
			name: "valid request",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/users/123?format=json", nil)
				return req
			}(),
			expectValid: true,
			expectError: false,
		},
		{
			name: "invalid query parameter",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/users/123?format=yaml", nil)
				return req
			}(),
			expectValid: false,
			expectError: false,
		},
		{
			name: "endpoint not found",
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/nonexistent", nil)
				return req
			}(),
			expectValid: true, // In non-strict mode, missing endpoints are warnings
			expectError: false,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ValidateRequest(ctx, tt.request)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v. Errors: %v",
					tt.expectValid, result.Valid, result.Errors)
			}

			// Check that timestamp is set
			if result.Timestamp.IsZero() {
				t.Error("Expected timestamp to be set")
			}
		})
	}
}

func TestRequestValidator_PathParameterValidation(t *testing.T) {
	spec := &openapi.Specification{
		Version: "3.0.0",
		Paths: map[string]openapi.PathItem{
			"/users/{id}": {
				GET: &openapi.Operation{
					Parameters: []openapi.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema: &openapi.Schema{
								Type:    "integer",
								Minimum: func(f float64) *float64 { return &f }(1),
								Maximum: func(f float64) *float64 { return &f }(999999),
							},
						},
					},
					Responses: map[string]openapi.Response{
						"200": {Description: "Success"},
					},
				},
			},
		},
	}

	config := &Config{
		Enabled:      true,
		ValidatePath: true,
	}

	validator := NewRequestValidator(spec, config)
	ctx := context.Background()

	tests := []struct {
		name        string
		url         string
		expectValid bool
	}{
		{"valid integer", "/users/123", true},
		{"invalid integer", "/users/abc", false},
		{"integer too small", "/users/0", false},
		{"integer too large", "/users/9999999", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			result, err := validator.ValidateRequest(ctx, req)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v. Errors: %v",
					tt.expectValid, result.Valid, result.Errors)
			}
		})
	}
}

func TestRequestValidator_JSONBodyValidation(t *testing.T) {
	spec := &openapi.Specification{
		Version: "3.0.0",
		Paths: map[string]openapi.PathItem{
			"/users": {
				POST: &openapi.Operation{
					RequestBody: &openapi.RequestBody{
						Required: true,
						Content: map[string]openapi.MediaTypeObject{
							"application/json": {
								Schema: &openapi.Schema{
									Type: "object",
									Properties: map[string]*openapi.Schema{
										"name": {
											Type:      "string",
											MinLength: func(i int) *int { return &i }(2),
											MaxLength: func(i int) *int { return &i }(50),
										},
										"age": {
											Type:    "integer",
											Minimum: func(f float64) *float64 { return &f }(0),
											Maximum: func(f float64) *float64 { return &f }(150),
										},
									},
									Required: []string{"name"},
								},
							},
						},
					},
					Responses: map[string]openapi.Response{
						"201": {Description: "Created"},
					},
				},
			},
		},
	}

	config := &Config{
		Enabled:      true,
		ValidateBody: true,
	}

	validator := NewRequestValidator(spec, config)
	ctx := context.Background()

	tests := []struct {
		name        string
		body        string
		contentType string
		expectValid bool
	}{
		{
			name:        "valid JSON",
			body:        `{"name": "John Doe", "age": 30}`,
			contentType: "application/json",
			expectValid: true,
		},
		{
			name:        "missing required field",
			body:        `{"age": 30}`,
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "invalid JSON",
			body:        `{"name": "John Doe", "age":}`,
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "field too short",
			body:        `{"name": "J", "age": 30}`,
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "age out of range",
			body:        `{"name": "John Doe", "age": 200}`,
			contentType: "application/json",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/users", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			result, err := validator.ValidateRequest(ctx, req)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v. Errors: %v",
					tt.expectValid, result.Valid, result.Errors)
			}
		})
	}
}

func TestRequestValidator_StrictMode(t *testing.T) {
	spec := &openapi.Specification{
		Version: "3.0.0",
		Paths: map[string]openapi.PathItem{
			"/users": {
				GET: &openapi.Operation{
					Responses: map[string]openapi.Response{
						"200": {Description: "Success"},
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		strictMode  bool
		url         string
		expectValid bool
		expectError bool
	}{
		{
			name:        "non-strict mode - missing endpoint as warning",
			strictMode:  false,
			url:         "/nonexistent",
			expectValid: true,
			expectError: false,
		},
		{
			name:        "strict mode - missing endpoint as error",
			strictMode:  true,
			url:         "/nonexistent",
			expectValid: false,
			expectError: false,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Enabled:    true,
				StrictMode: tt.strictMode,
			}

			validator := NewRequestValidator(spec, config)
			req, _ := http.NewRequest("GET", tt.url, nil)

			result, err := validator.ValidateRequest(ctx, req)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v. Errors: %v, Warnings: %v",
					tt.expectValid, result.Valid, result.Errors, result.Warnings)
			}

			if tt.strictMode && !tt.expectValid && len(result.Errors) == 0 {
				t.Error("Expected errors in strict mode")
			}

			if !tt.strictMode && tt.expectValid && len(result.Warnings) == 0 {
				t.Error("Expected warnings in non-strict mode")
			}
		})
	}
}

func BenchmarkRequestValidator_ValidateRequest(b *testing.B) {
	spec := &openapi.Specification{
		Version: "3.0.0",
		Paths: map[string]openapi.PathItem{
			"/users/{id}": {
				GET: &openapi.Operation{
					Parameters: []openapi.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   &openapi.Schema{Type: "string"},
						},
					},
					Responses: map[string]openapi.Response{
						"200": {Description: "Success"},
					},
				},
			},
		},
	}

	config := &Config{Enabled: true}
	validator := NewRequestValidator(spec, config)
	ctx := context.Background()

	req, _ := http.NewRequest("GET", "/users/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateRequest(ctx, req)
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}
