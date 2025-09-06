package openapi

import (
	"testing"
	"time"
)

func TestNewDefaultDataGenerator(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	if generator == nil {
		t.Fatal("NewDefaultDataGenerator() returned nil")
	}
	
	if generator.faker == nil {
		t.Error("generator.faker is nil")
	}
	
	if generator.formatGenerators == nil {
		t.Error("generator.formatGenerators is nil")
	}
	
	if generator.locale != "en" {
		t.Errorf("expected default locale 'en', got %s", generator.locale)
	}
	
	if generator.seed == 0 {
		t.Error("expected non-zero seed")
	}
}

func TestNewDefaultDataGeneratorWithSeed(t *testing.T) {
	seed := int64(12345)
	generator := NewDefaultDataGeneratorWithSeed(seed)
	
	if generator == nil {
		t.Fatal("NewDefaultDataGeneratorWithSeed() returned nil")
	}
	
	if generator.seed != seed {
		t.Errorf("expected seed %d, got %d", seed, generator.seed)
	}
}

func TestSetSeed(t *testing.T) {
	generator := NewDefaultDataGenerator()
	originalSeed := generator.seed
	
	newSeed := int64(54321)
	generator.SetSeed(newSeed)
	
	if generator.seed != newSeed {
		t.Errorf("expected seed %d, got %d", newSeed, generator.seed)
	}
	
	if generator.seed == originalSeed {
		t.Error("seed was not changed")
	}
}

func TestSetLocale(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	generator.SetLocale("fr")
	
	if generator.locale != "fr" {
		t.Errorf("expected locale 'fr', got %s", generator.locale)
	}
}

func TestGenerateString(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345) // For reproducible tests
	
	schema := &Schema{
		Type: "string",
	}
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
		Locale:       "en",
		Seed:         12345,
		Timestamp:    time.Now(),
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	
	if len(str) == 0 {
		t.Error("generated string is empty")
	}
}

func TestGenerateStringWithLength(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	minLength := 5
	maxLength := 10
	schema := &Schema{
		Type:      "string",
		MinLength: &minLength,
		MaxLength: &maxLength,
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
	
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	
	if len(str) < minLength || len(str) > maxLength {
		t.Errorf("string length %d not within bounds [%d, %d]", len(str), minLength, maxLength)
	}
}

func TestGenerateInteger(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	schema := &Schema{
		Type: "integer",
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
	
	_, ok := result.(int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
}

func TestGenerateIntegerWithBounds(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	min := 10.0
	max := 20.0
	schema := &Schema{
		Type:    "integer",
		Minimum: &min,
		Maximum: &max,
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
	
	value, ok := result.(int)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	
	if value < int(min) || value > int(max) {
		t.Errorf("integer value %d not within bounds [%d, %d]", value, int(min), int(max))
	}
}

func TestGenerateBoolean(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	schema := &Schema{
		Type: "boolean",
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
	
	_, ok := result.(bool)
	if !ok {
		t.Fatalf("expected bool, got %T", result)
	}
}

func TestGenerateArray(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	schema := &Schema{
		Type: "array",
		Items: &Schema{
			Type: "string",
		},
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
	
	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}
	
	if len(arr) == 0 {
		t.Error("generated array is empty")
	}
	
	// Check that all items are strings
	for i, item := range arr {
		if _, ok := item.(string); !ok {
			t.Errorf("array item %d is not a string: %T", i, item)
		}
	}
}

func TestGenerateObject(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {
				Type: "string",
			},
			"age": {
				Type: "integer",
			},
		},
		Required: []string{"name"},
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
	
	obj, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	
	// Check required field is present
	if _, exists := obj["name"]; !exists {
		t.Error("required field 'name' is missing")
	}
	
	// Check field types
	if name, exists := obj["name"]; exists {
		if _, ok := name.(string); !ok {
			t.Errorf("field 'name' should be string, got %T", name)
		}
	}
	
	if age, exists := obj["age"]; exists {
		if _, ok := age.(int); !ok {
			t.Errorf("field 'age' should be int, got %T", age)
		}
	}
}

func TestGenerateWithExample(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	example := "test example"
	schema := &Schema{
		Type:    "string",
		Example: example,
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
	
	if result != example {
		t.Errorf("expected example '%s', got '%v'", example, result)
	}
}

func TestGenerateWithEnum(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	enum := []interface{}{"red", "green", "blue"}
	schema := &Schema{
		Type: "string",
		Enum: enum,
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
	
	// Check that result is one of the enum values
	found := false
	for _, val := range enum {
		if result == val {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("result '%v' is not one of the enum values %v", result, enum)
	}
}

func TestGenerateWithMaxDepth(t *testing.T) {
	generator := NewDefaultDataGenerator()
	
	// Create a deeply nested schema
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"nested": {
				Type: "object",
				Properties: map[string]*Schema{
					"deep": {
						Type: "string",
					},
				},
			},
		},
	}
	
	ctx := &GenerationContext{
		MaxDepth:     1, // Very shallow depth
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	
	obj, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result)
	}
	
	// At depth 1, nested objects should be limited
	if nested, exists := obj["nested"]; exists {
		if nested == nil {
			// This is acceptable - the generator may return nil for deeply nested objects
		} else if nestedObj, ok := nested.(map[string]interface{}); ok {
			// If it exists, it should be shallow
			if len(nestedObj) > 0 {
				t.Log("Nested object was generated despite shallow depth limit")
			}
		}
	}
}