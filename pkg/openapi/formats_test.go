package openapi

import (
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFormatGenerators(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345) // For reproducible tests
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	tests := []struct {
		name     string
		format   string
		expected string // regex pattern or exact match
		validate func(interface{}) bool
	}{
		{
			name:   "email",
			format: FormatEmail,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// Simple email validation
				return strings.Contains(str, "@") && strings.Contains(str, ".")
			},
		},
		{
			name:   "uuid",
			format: FormatUUID,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// UUID v4 pattern
				uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
				return uuidRegex.MatchString(str)
			},
		},
		{
			name:   "date-time",
			format: FormatDateTime,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// Try to parse as RFC3339
				_, err := time.Parse(time.RFC3339, str)
				return err == nil
			},
		},
		{
			name:   "date",
			format: FormatDate,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// Try to parse as date
				_, err := time.Parse("2006-01-02", str)
				return err == nil
			},
		},
		{
			name:   "time",
			format: FormatTime,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// Check time format HH:MM:SS
				timeRegex := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
				return timeRegex.MatchString(str)
			},
		},
		{
			name:   "ipv4",
			format: FormatIPv4,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				ip := net.ParseIP(str)
				return ip != nil && ip.To4() != nil
			},
		},
		{
			name:   "ipv6",
			format: FormatIPv6,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				ip := net.ParseIP(str)
				return ip != nil && ip.To4() == nil
			},
		},
		{
			name:   "uri",
			format: FormatURI,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
			},
		},
		{
			name:   "hostname",
			format: FormatHostname,
			validate: func(v interface{}) bool {
				str, ok := v.(string)
				if !ok {
					return false
				}
				// Simple hostname validation
				return len(str) > 0 && strings.Contains(str, ".")
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &Schema{
				Type:   "string",
				Format: tt.format,
			}
			
			result, err := generator.Generate(schema, ctx)
			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}
			
			if !tt.validate(result) {
				t.Errorf("Generated value '%v' does not match expected format %s", result, tt.format)
			}
		})
	}
}

func TestNumericFormats(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	t.Run("float", func(t *testing.T) {
		schema := &Schema{
			Type:   "number",
			Format: FormatFloat,
		}
		
		result, err := generator.Generate(schema, ctx)
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		
		_, ok := result.(float32)
		if !ok {
			t.Errorf("expected float32, got %T", result)
		}
	})
	
	t.Run("double", func(t *testing.T) {
		schema := &Schema{
			Type:   "number",
			Format: FormatDouble,
		}
		
		result, err := generator.Generate(schema, ctx)
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		
		_, ok := result.(float64)
		if !ok {
			t.Errorf("expected float64, got %T", result)
		}
	})
	
	t.Run("int32", func(t *testing.T) {
		schema := &Schema{
			Type:   "integer",
			Format: FormatInt32,
		}
		
		result, err := generator.Generate(schema, ctx)
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		
		_, ok := result.(int32)
		if !ok {
			t.Errorf("expected int32, got %T", result)
		}
	})
	
	t.Run("int64", func(t *testing.T) {
		schema := &Schema{
			Type:   "integer",
			Format: FormatInt64,
		}
		
		result, err := generator.Generate(schema, ctx)
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		
		_, ok := result.(int64)
		if !ok {
			t.Errorf("expected int64, got %T", result)
		}
	})
}

func TestPasswordFormat(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	minLength := 8
	maxLength := 16
	schema := &Schema{
		Type:      "string",
		Format:    FormatPassword,
		MinLength: &minLength,
		MaxLength: &maxLength,
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	
	password, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	
	if len(password) < minLength || len(password) > maxLength {
		t.Errorf("password length %d not within bounds [%d, %d]", len(password), minLength, maxLength)
	}
	
	// Check password complexity (should have different character types)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	
	if !hasLower && !hasUpper && !hasDigit {
		t.Error("password should contain at least some variety of characters")
	}
}

func TestCustomFormatRegistration(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	// Register custom format
	customFormat := "test-format"
	generator.RegisterCustomFormat(customFormat, func(schema *Schema, ctx *GenerationContext) (interface{}, error) {
		return "custom-value", nil
	})
	
	// Test that the format is registered
	if !generator.HasFormatGenerator(customFormat) {
		t.Error("custom format was not registered")
	}
	
	// Test generation with custom format
	schema := &Schema{
		Type:   "string",
		Format: customFormat,
	}
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	
	if result != "custom-value" {
		t.Errorf("expected 'custom-value', got '%v'", result)
	}
}

func TestGetRegisteredFormats(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	formats := generator.GetRegisteredFormats()
	
	if len(formats) == 0 {
		t.Error("no formats registered")
	}
	
	// Check for some standard formats
	expectedFormats := []string{FormatEmail, FormatUUID, FormatDateTime, FormatIPv4}
	for _, expected := range expectedFormats {
		found := false
		for _, format := range formats {
			if format == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected format '%s' not found in registered formats", expected)
		}
	}
}

func TestGetFormatGeneratorInfo(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	infos := generator.GetFormatGeneratorInfo()
	
	if len(infos) == 0 {
		t.Error("no format generator info returned")
	}
	
	// Check structure of info
	for _, info := range infos {
		if info.Format == "" {
			t.Error("format info has empty format")
		}
		if info.Description == "" {
			t.Error("format info has empty description")
		}
		if info.Example == "" {
			t.Error("format info has empty example")
		}
	}
}

func TestRemoveFormatGenerator(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	// Check that email format exists
	if !generator.HasFormatGenerator(FormatEmail) {
		t.Fatal("email format should be registered by default")
	}
	
	// Remove email format
	generator.RemoveFormatGenerator(FormatEmail)
	
	// Check that it's removed
	if generator.HasFormatGenerator(FormatEmail) {
		t.Error("email format should have been removed")
	}
	
	// Try to generate with removed format - should fall back to basic string generation
	schema := &Schema{
		Type:   "string",
		Format: FormatEmail,
	}
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	
	// Should generate a basic string since email format was removed
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	
	// Should not be a valid email since format generator was removed
	if strings.Contains(str, "@") {
		t.Log("Generated value looks like email despite format removal - may be coincidental")
	}
}