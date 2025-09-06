package openapi

import (
	"encoding/json"
	"testing"
)

func TestIntegrationWithSimpleSpec(t *testing.T) {
	// Create a simple OpenAPI specification
	spec := &Specification{
		Version: "3.0.0",
		Info: InfoObject{
			Title:       "Test API",
			Description: "A simple test API",
			Version:     "1.0.0",
		},
		Paths: map[string]PathItem{
			"/users": {
				GET: &Operation{
					OperationID: "getUsers",
					Summary:     "Get all users",
					Responses: map[string]Response{
						"200": {
							Description: "Successful response",
							Content: map[string]MediaTypeObject{
								"application/json": {
									Schema: &Schema{
										Type: "array",
										Items: &Schema{
											Type: "object",
											Properties: map[string]*Schema{
												"id": {
													Type:   "integer",
													Format: "int64",
												},
												"name": {
													Type:      "string",
													MinLength: intPtr(1),
													MaxLength: intPtr(100),
												},
												"email": {
													Type:   "string",
													Format: "email",
												},
												"age": {
													Type:    "integer",
													Minimum: floatPtr(0),
													Maximum: floatPtr(150),
												},
												"active": {
													Type: "boolean",
												},
											},
											Required: []string{"id", "name", "email"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/users/{id}": {
				GET: &Operation{
					OperationID: "getUserById",
					Summary:     "Get user by ID",
					Parameters: []Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema: &Schema{
								Type:   "integer",
								Format: "int64",
							},
						},
					},
					Responses: map[string]Response{
						"200": {
							Description: "Successful response",
							Content: map[string]MediaTypeObject{
								"application/json": {
									Schema: &Schema{
										Type: "object",
										Properties: map[string]*Schema{
											"id": {
												Type:   "integer",
												Format: "int64",
											},
											"name": {
												Type: "string",
											},
											"email": {
												Type:   "string",
												Format: "email",
											},
											"profile": {
												Type: "object",
												Properties: map[string]*Schema{
													"bio": {
														Type: "string",
													},
													"website": {
														Type:   "string",
														Format: "uri",
													},
													"location": {
														Type: "string",
													},
												},
											},
										},
										Required: []string{"id", "name", "email"},
									},
								},
							},
						},
						"404": {
							Description: "User not found",
							Content: map[string]MediaTypeObject{
								"application/json": {
									Schema: &Schema{
										Type: "object",
										Properties: map[string]*Schema{
											"error": {
												Type: "string",
											},
											"message": {
												Type: "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Schemas: make(map[string]*Schema),
	}
	
	// Create generator
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345) // For reproducible tests
	
	// Test generating data for the array response
	arraySchema := spec.Paths["/users"].GET.Responses["200"].Content["application/json"].Schema
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	result, err := generator.Generate(arraySchema, ctx)
	if err != nil {
		t.Fatalf("Failed to generate data: %v", err)
	}
	
	// Verify it's an array
	users, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected array, got %T", result)
	}
	
	if len(users) == 0 {
		t.Error("Generated empty users array")
	}
	
	// Check the first user
	if len(users) > 0 {
		user, ok := users[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected user object, got %T", users[0])
		}
		
		// Check required fields
		requiredFields := []string{"id", "name", "email"}
		for _, field := range requiredFields {
			if _, exists := user[field]; !exists {
				t.Errorf("Required field '%s' missing from user", field)
			}
		}
		
		// Check field types
		if id, exists := user["id"]; exists {
			if _, ok := id.(int64); !ok {
				t.Errorf("User ID should be int64, got %T", id)
			}
		}
		
		if name, exists := user["name"]; exists {
			if _, ok := name.(string); !ok {
				t.Errorf("User name should be string, got %T", name)
			}
		}
		
		if email, exists := user["email"]; exists {
			if emailStr, ok := email.(string); ok {
				if !isValidEmail(emailStr) {
					t.Errorf("Generated email '%s' is not valid", emailStr)
				}
			} else {
				t.Errorf("User email should be string, got %T", email)
			}
		}
		
		if age, exists := user["age"]; exists {
			if ageInt, ok := age.(int); ok {
				if ageInt < 0 || ageInt > 150 {
					t.Errorf("User age %d is out of valid range [0, 150]", ageInt)
				}
			} else {
				t.Errorf("User age should be int, got %T", age)
			}
		}
	}
	
	// Test generating data for the single user response
	userSchema := spec.Paths["/users/{id}"].GET.Responses["200"].Content["application/json"].Schema
	
	ctx.Reset()
	singleUserResult, err := generator.Generate(userSchema, ctx)
	if err != nil {
		t.Fatalf("Failed to generate single user data: %v", err)
	}
	
	singleUser, ok := singleUserResult.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user object, got %T", singleUserResult)
	}
	
	// Check nested profile object
	if profile, exists := singleUser["profile"]; exists && profile != nil {
		profileObj, ok := profile.(map[string]interface{})
		if !ok {
			t.Errorf("Profile should be object, got %T", profile)
		} else {
			if website, exists := profileObj["website"]; exists {
				if websiteStr, ok := website.(string); ok {
					if !isValidURL(websiteStr) {
						t.Errorf("Generated website URL '%s' is not valid", websiteStr)
					}
				}
			}
		}
	}
	
	// Test error response generation
	errorSchema := spec.Paths["/users/{id}"].GET.Responses["404"].Content["application/json"].Schema
	
	ctx.Reset()
	errorResult, err := generator.Generate(errorSchema, ctx)
	if err != nil {
		t.Fatalf("Failed to generate error data: %v", err)
	}
	
	errorObj, ok := errorResult.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error object, got %T", errorResult)
	}
	
	if len(errorObj) == 0 {
		t.Error("Generated empty error object")
	}
}

func TestJSONSerialization(t *testing.T) {
	generator := NewDefaultDataGenerator()
	generator.SetSeed(12345)
	
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"id": {
				Type:   "integer",
				Format: "int64",
			},
			"name": {
				Type: "string",
			},
			"tags": {
				Type: "array",
				Items: &Schema{
					Type: "string",
				},
			},
			"metadata": {
				Type: "object",
				Properties: map[string]*Schema{
					"created_at": {
						Type:   "string",
						Format: "date-time",
					},
					"updated_at": {
						Type:   "string",
						Format: "date-time",
					},
				},
			},
		},
		Required: []string{"id", "name"},
	}
	
	ctx := &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	result, err := generator.Generate(schema, ctx)
	if err != nil {
		t.Fatalf("Failed to generate data: %v", err)
	}
	
	// Test JSON serialization
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}
	
	if len(jsonBytes) == 0 {
		t.Error("Generated empty JSON")
	}
	
	// Test JSON deserialization
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	// Basic validation of round-trip
	if len(unmarshaled) == 0 {
		t.Error("Unmarshaled object is empty")
	}
	
	t.Logf("Generated JSON: %s", string(jsonBytes))
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func isValidEmail(email string) bool {
	// Very basic email validation
	return len(email) > 0 && contains(email, "@") && contains(email, ".")
}

func isValidURL(url string) bool {
	// Very basic URL validation
	return len(url) > 0 && (hasPrefix(url, "http://") || hasPrefix(url, "https://"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOfSubstring(s, substr) >= 0)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}