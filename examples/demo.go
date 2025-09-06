package main

import (
	"encoding/json"
	"fmt"
	"log"

	"vanta/pkg/openapi"
)

func main() {
	fmt.Println("Vanta OpenAPI Mock Data Generator Demo")
	fmt.Println("====================================")
	
	// Create a sample OpenAPI schema
	userSchema := &openapi.Schema{
		Type: "object",
		Properties: map[string]*openapi.Schema{
			"id": {
				Type:   "integer",
				Format: "int64",
			},
			"name": {
				Type:      "string",
				MinLength: intPtr(2),
				MaxLength: intPtr(50),
			},
			"email": {
				Type:   "string",
				Format: "email",
			},
			"age": {
				Type:    "integer",
				Minimum: floatPtr(18),
				Maximum: floatPtr(100),
			},
			"active": {
				Type: "boolean",
			},
			"profile": {
				Type: "object",
				Properties: map[string]*openapi.Schema{
					"bio": {
						Type: "string",
					},
					"website": {
						Type:   "string",
						Format: "uri",
					},
					"created_at": {
						Type:   "string",
						Format: "date-time",
					},
				},
			},
			"tags": {
				Type:     "array",
				MinItems: intPtr(1),
				MaxItems: intPtr(5),
				Items: &openapi.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"id", "name", "email"},
	}
	
	// Create generator
	generator := openapi.NewDefaultDataGeneratorWithSeed(12345)
	
	// Create generation context
	ctx := &openapi.GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
		Locale:       "en",
		Seed:         12345,
	}
	
	fmt.Println("\n1. Generating a single user:")
	fmt.Println("---------------------------")
	
	// Generate user data
	userData, err := generator.Generate(userSchema, ctx)
	if err != nil {
		log.Fatalf("Failed to generate user data: %v", err)
	}
	
	// Pretty print JSON
	userJSON, err := json.MarshalIndent(userData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal user data: %v", err)
	}
	
	fmt.Println(string(userJSON))
	
	fmt.Println("\n2. Generating an array of users:")
	fmt.Println("--------------------------------")
	
	// Create array schema
	usersSchema := &openapi.Schema{
		Type:     "array",
		MinItems: intPtr(3),
		MaxItems: intPtr(3),
		Items:    userSchema,
	}
	
	// Reset context for new generation
	resetContext(ctx)
	
	// Generate users array
	usersData, err := generator.Generate(usersSchema, ctx)
	if err != nil {
		log.Fatalf("Failed to generate users array: %v", err)
	}
	
	usersJSON, err := json.MarshalIndent(usersData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal users data: %v", err)
	}
	
	fmt.Println(string(usersJSON))
	
	fmt.Println("\n3. Testing different formats:")
	fmt.Println("----------------------------")
	
	formatsSchema := &openapi.Schema{
		Type: "object",
		Properties: map[string]*openapi.Schema{
			"uuid": {
				Type:   "string",
				Format: "uuid",
			},
			"email": {
				Type:   "string",
				Format: "email",
			},
			"date": {
				Type:   "string",
				Format: "date",
			},
			"datetime": {
				Type:   "string",
				Format: "date-time",
			},
			"ipv4": {
				Type:   "string",
				Format: "ipv4",
			},
			"url": {
				Type:   "string",
				Format: "uri",
			},
		},
	}
	
	resetContext(ctx)
	formatsData, err := generator.Generate(formatsSchema, ctx)
	if err != nil {
		log.Fatalf("Failed to generate formats data: %v", err)
	}
	
	formatsJSON, err := json.MarshalIndent(formatsData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal formats data: %v", err)
	}
	
	fmt.Println(string(formatsJSON))
	
	fmt.Println("\n4. Testing with examples and enums:")
	fmt.Println("-----------------------------------")
	
	enumSchema := &openapi.Schema{
		Type: "object",
		Properties: map[string]*openapi.Schema{
			"status": {
				Type: "string",
				Enum: []interface{}{"active", "inactive", "pending"},
			},
			"priority": {
				Type: "string",
				Enum: []interface{}{"low", "medium", "high", "urgent"},
			},
			"welcome_message": {
				Type:    "string",
				Example: "Welcome to our amazing service!",
			},
		},
	}
	
	resetContext(ctx)
	enumData, err := generator.Generate(enumSchema, ctx)
	if err != nil {
		log.Fatalf("Failed to generate enum data: %v", err)
	}
	
	enumJSON, err := json.MarshalIndent(enumData, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal enum data: %v", err)
	}
	
	fmt.Println(string(enumJSON))
	
	fmt.Println("\n5. Format generator information:")
	fmt.Println("-------------------------------")
	
	formats := generator.GetRegisteredFormats()
	fmt.Printf("Registered formats (%d): %v\n", len(formats), formats[:10]) // Show first 10
	
	fmt.Println("\nDemo completed successfully!")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("- ✓ Generates realistic mock data for all OpenAPI types")
	fmt.Println("- ✓ Respects schema constraints (min/max, length, patterns)")
	fmt.Println("- ✓ Supports all standard OpenAPI formats")
	fmt.Println("- ✓ Handles complex nested objects and arrays")
	fmt.Println("- ✓ Uses examples and enum values when available")
	fmt.Println("- ✓ Prevents infinite recursion with depth limiting")
	fmt.Println("- ✓ Provides deterministic output with seeds")
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

// resetContext resets the generation context
func resetContext(ctx *openapi.GenerationContext) {
	ctx.CurrentDepth = 0
	ctx.Parent = ""
	ctx.Required = false
	ctx.Visited = make(map[string]bool)
	ctx.ArraySizes = make(map[string]int)
}