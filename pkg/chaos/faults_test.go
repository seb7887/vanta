package chaos

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zaptest"
)

func TestNewErrorInjector(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	
	injector := NewErrorInjector(logger, rng)
	assert.NotNil(t, injector)
	assert.Equal(t, "error", injector.Type())
}

func TestErrorInjectorValidate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid_params",
			params: map[string]interface{}{
				"error_codes": []interface{}{500, 502, 503},
			},
			wantErr: false,
		},
		{
			name: "valid_params_with_custom_body",
			params: map[string]interface{}{
				"error_codes": []interface{}{500, 502},
				"custom_body": `{"error": "Custom error message"}`,
			},
			wantErr: false,
		},
		{
			name: "missing_error_codes",
			params: map[string]interface{}{
				"custom_body": "test",
			},
			wantErr: true,
		},
		{
			name: "error_codes_not_slice",
			params: map[string]interface{}{
				"error_codes": 500,
			},
			wantErr: true,
		},
		{
			name: "error_codes_empty_slice",
			params: map[string]interface{}{
				"error_codes": []interface{}{},
			},
			wantErr: false, // Empty slice is valid but will cause runtime issues
		},
		{
			name: "invalid_error_code_type",
			params: map[string]interface{}{
				"error_codes": []interface{}{"500"},
			},
			wantErr: true,
		},
		{
			name: "error_code_too_low",
			params: map[string]interface{}{
				"error_codes": []interface{}{399},
			},
			wantErr: true,
		},
		{
			name: "error_code_too_high",
			params: map[string]interface{}{
				"error_codes": []interface{}{600},
			},
			wantErr: true,
		},
		{
			name: "mixed_valid_invalid_codes",
			params: map[string]interface{}{
				"error_codes": []interface{}{500, 399},
			},
			wantErr: true,
		},
		{
			name: "custom_body_not_string",
			params: map[string]interface{}{
				"error_codes": []interface{}{500},
				"custom_body": 123,
			},
			wantErr: true,
		},
		{
			name: "float_error_codes",
			params: map[string]interface{}{
				"error_codes": []interface{}{500.0, 502.0},
			},
			wantErr: false,
		},
		{
			name: "boundary_error_codes",
			params: map[string]interface{}{
				"error_codes": []interface{}{400, 599},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorInjectorInject(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test/path")
	
	tests := []struct {
		name           string
		params         map[string]interface{}
		expectedCodes  []int
		expectedBody   string
		customBodySet  bool
	}{
		{
			name: "single_error_code",
			params: map[string]interface{}{
				"error_codes": []interface{}{500},
			},
			expectedCodes: []int{500},
			customBodySet: false,
		},
		{
			name: "multiple_error_codes",
			params: map[string]interface{}{
				"error_codes": []interface{}{500, 502, 503},
			},
			expectedCodes: []int{500, 502, 503},
			customBodySet: false,
		},
		{
			name: "with_custom_body",
			params: map[string]interface{}{
				"error_codes": []interface{}{500},
				"custom_body": `{"error": "Custom error"}`,
			},
			expectedCodes: []int{500},
			expectedBody:  `{"error": "Custom error"}`,
			customBodySet: true,
		},
		{
			name: "empty_custom_body",
			params: map[string]interface{}{
				"error_codes": []interface{}{404},
				"custom_body": "",
			},
			expectedCodes: []int{404},
			customBodySet: false, // Empty custom body should fall back to default
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset context
			ctx.Response.Reset()
			
			err := injector.Inject(ctx, tt.params)
			require.NoError(t, err)
			
			// Check status code is one of expected codes
			statusCode := ctx.Response.StatusCode()
			assert.Contains(t, tt.expectedCodes, statusCode)
			
			// Check content type
			contentType := string(ctx.Response.Header.ContentType())
			assert.Equal(t, "application/json", contentType)
			
			// Check body
			body := string(ctx.Response.Body())
			if tt.customBodySet {
				assert.Equal(t, tt.expectedBody, body)
			} else {
				assert.NotEmpty(t, body)
				assert.Contains(t, body, "error")
				assert.Contains(t, body, "message")
			}
		})
	}
}

func TestErrorInjectorInjectInvalidParams(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	
	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "error_codes_not_slice",
			params: map[string]interface{}{
				"error_codes": "not a slice",
			},
		},
		{
			name: "error_codes_contains_invalid_type",
			params: map[string]interface{}{
				"error_codes": []interface{}{"invalid"},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Inject(ctx, tt.params)
			assert.Error(t, err)
		})
	}
}

func TestParseErrorCodes(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	tests := []struct {
		name      string
		input     interface{}
		expected  []int
		wantErr   bool
	}{
		{
			name:     "int_slice",
			input:    []interface{}{500, 502, 503},
			expected: []int{500, 502, 503},
			wantErr:  false,
		},
		{
			name:     "float_slice",
			input:    []interface{}{500.0, 502.0},
			expected: []int{500, 502},
			wantErr:  false,
		},
		{
			name:     "mixed_numeric_slice",
			input:    []interface{}{500, 502.0},
			expected: []int{500, 502},
			wantErr:  false,
		},
		{
			name:    "not_slice",
			input:   500,
			wantErr: true,
		},
		{
			name:    "string_in_slice",
			input:   []interface{}{"500"},
			wantErr: true,
		},
		{
			name:     "empty_slice",
			input:    []interface{}{},
			expected: []int{},
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := injector.parseErrorCodes(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetDefaultErrorBody(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	tests := []struct {
		statusCode int
		contains   []string
	}{
		{400, []string{"Bad Request", "malformed syntax"}},
		{401, []string{"Unauthorized", "authentication"}},
		{403, []string{"Forbidden", "refusing to fulfill"}},
		{404, []string{"Not Found", "Request-URI"}},
		{500, []string{"Internal Server Error", "unexpected condition"}},
		{502, []string{"Bad Gateway", "invalid response"}},
		{503, []string{"Service Unavailable", "temporary overloading"}},
		{999, []string{"HTTP Error 999", "error occurred"}}, // Unknown code
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.statusCode)), func(t *testing.T) {
			body := injector.getDefaultErrorBody(tt.statusCode)
			assert.NotEmpty(t, body)
			for _, expected := range tt.contains {
				assert.Contains(t, body, expected)
			}
		})
	}
}

func TestGetCustomBody(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	tests := []struct {
		name       string
		params     map[string]interface{}
		statusCode int
		expected   string
		isDefault  bool
	}{
		{
			name: "with_custom_body",
			params: map[string]interface{}{
				"custom_body": `{"error": "Custom message"}`,
			},
			statusCode: 500,
			expected:   `{"error": "Custom message"}`,
			isDefault:  false,
		},
		{
			name: "empty_custom_body",
			params: map[string]interface{}{
				"custom_body": "",
			},
			statusCode: 404,
			isDefault:  true,
		},
		{
			name:       "no_custom_body",
			params:     map[string]interface{}{},
			statusCode: 500,
			isDefault:  true,
		},
		{
			name: "custom_body_wrong_type",
			params: map[string]interface{}{
				"custom_body": 123,
			},
			statusCode: 500,
			isDefault:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injector.getCustomBody(tt.params, tt.statusCode)
			if tt.isDefault {
				assert.Contains(t, result, "error")
				assert.Contains(t, result, "message")
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestErrorInjectorTypeMethod(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	assert.Equal(t, "error", injector.Type())
}

func BenchmarkErrorInjectorInject(b *testing.B) {
	logger := zaptest.NewLogger(b)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	params := map[string]interface{}{
		"error_codes": []interface{}{500, 502, 503},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Response.Reset()
		_ = injector.Inject(ctx, params)
	}
}

func BenchmarkParseErrorCodes(b *testing.B) {
	logger := zaptest.NewLogger(b)
	rng := rand.New(rand.NewSource(1))
	injector := NewErrorInjector(logger, rng)
	
	errorCodes := []interface{}{500, 502, 503, 504}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = injector.parseErrorCodes(errorCodes)
	}
}