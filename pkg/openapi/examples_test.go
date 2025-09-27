package openapi

import (
	"testing"
)

func TestSelectExample(t *testing.T) {
	generator := NewDefaultDataGenerator()

	// Test schema with multiple examples
	schema := &Schema{
		Type: "object",
		Examples: map[string]ExampleObject{
			"user_success": {
				Summary: "Successful user",
				Value: map[string]interface{}{
					"id":   1,
					"name": "John Doe",
					"status": "active",
				},
			},
			"user_error": {
				Summary: "Error user",
				Value: map[string]interface{}{
					"id":   1,
					"name": "John Doe",
					"status": "inactive",
					"error": "suspended",
				},
			},
		},
	}

	t.Run("select specific example", func(t *testing.T) {
		ctx := &GenerationContext{
			RequestedExample: "user_success",
		}

		result := generator.selectExample(schema, ctx)
		if result == nil {
			t.Fatal("Expected example to be selected, got nil")
		}

		userMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{}, got different type")
		}

		if userMap["status"] != "active" {
			t.Errorf("Expected status 'active', got '%v'", userMap["status"])
		}
	})

	t.Run("select different example", func(t *testing.T) {
		ctx := &GenerationContext{
			RequestedExample: "user_error",
		}

		result := generator.selectExample(schema, ctx)
		if result == nil {
			t.Fatal("Expected example to be selected, got nil")
		}

		userMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{}, got different type")
		}

		if userMap["status"] != "inactive" {
			t.Errorf("Expected status 'inactive', got '%v'", userMap["status"])
		}

		if userMap["error"] != "suspended" {
			t.Errorf("Expected error 'suspended', got '%v'", userMap["error"])
		}
	})

	t.Run("random example selection", func(t *testing.T) {
		ctx := &GenerationContext{
			RequestedExample: "random",
		}

		result := generator.selectExample(schema, ctx)
		if result == nil {
			t.Fatal("Expected example to be selected, got nil")
		}

		// Should return one of the available examples
		userMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{}, got different type")
		}

		status := userMap["status"]
		if status != "active" && status != "inactive" {
			t.Errorf("Expected status to be 'active' or 'inactive', got '%v'", status)
		}
	})

	t.Run("nonexistent example falls back to first", func(t *testing.T) {
		ctx := &GenerationContext{
			RequestedExample: "nonexistent",
		}

		result := generator.selectExample(schema, ctx)
		if result == nil {
			t.Fatal("Expected example to be selected, got nil")
		}

		// Should fall back to first available example
		userMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected map[string]interface{}, got different type")
		}

		if userMap["id"] != 1 {
			t.Errorf("Expected id to be 1, got '%v'", userMap["id"])
		}
	})

	t.Run("no examples returns nil", func(t *testing.T) {
		schemaNoExamples := &Schema{
			Type: "string",
		}

		ctx := &GenerationContext{
			RequestedExample: "any",
		}

		result := generator.selectExample(schemaNoExamples, ctx)
		if result != nil {
			t.Errorf("Expected nil when no examples available, got %v", result)
		}
	})

	t.Run("single example fallback", func(t *testing.T) {
		schemaSingleExample := &Schema{
			Type: "string",
			Example: "hello world",
		}

		ctx := &GenerationContext{
			RequestedExample: "any",
		}

		result := generator.selectExample(schemaSingleExample, ctx)
		if result != "hello world" {
			t.Errorf("Expected 'hello world', got '%v'", result)
		}
	})
}

func TestSelectRandomExample(t *testing.T) {
	generator := NewDefaultDataGenerator()

	examples := map[string]ExampleObject{
		"example1": {Value: "value1"},
		"example2": {Value: "value2"},
		"example3": {Value: "value3"},
	}

	t.Run("selects from available examples", func(t *testing.T) {
		result := generator.selectRandomExample(examples)
		if result == nil {
			t.Fatal("Expected a value, got nil")
		}

		// Should be one of the available values
		found := false
		for _, example := range examples {
			if result == example.Value {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Result '%v' not found in available examples", result)
		}
	})

	t.Run("returns nil for empty examples", func(t *testing.T) {
		emptyExamples := map[string]ExampleObject{}
		result := generator.selectRandomExample(emptyExamples)
		if result != nil {
			t.Errorf("Expected nil for empty examples, got '%v'", result)
		}
	})
}