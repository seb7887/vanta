package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/seb7887/vanta/pkg/config"
	"github.com/seb7887/vanta/pkg/openapi"
	"github.com/seb7887/vanta/pkg/state"
	"github.com/seb7887/vanta/pkg/validation"
)

// TestStateAndValidationIntegration tests the integration between state management and validation
func TestStateAndValidationIntegration(t *testing.T) {
	// Setup OpenAPI specification for a user management API
	spec := &openapi.Specification{
		Version: "3.0.0",
		Info: openapi.InfoObject{
			Title:   "User Management API",
			Version: "1.0.0",
		},
		Paths: map[string]openapi.PathItem{
			"/users": {
				POST: &openapi.Operation{
					OperationID: "createUser",
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
										"email": {
											Type:    "string",
											Pattern: `^[^\s@]+@[^\s@]+\.[^\s@]+$`,
										},
									},
									Required: []string{"name", "email"},
								},
							},
						},
					},
					Responses: map[string]openapi.Response{
						"201": {
							Description: "User created",
							Content: map[string]openapi.MediaTypeObject{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "object",
										Properties: map[string]*openapi.Schema{
											"id":    {Type: "string"},
											"name":  {Type: "string"},
											"email": {Type: "string"},
										},
										Required: []string{"id", "name", "email"},
									},
								},
							},
						},
						"400": {Description: "Invalid request"},
					},
				},
			},
			"/users/{id}": {
				GET: &openapi.Operation{
					OperationID: "getUser",
					Parameters: []openapi.Parameter{
						{
							Name:     "id",
							In:       "path",
							Required: true,
							Schema:   &openapi.Schema{Type: "string"},
						},
					},
					Responses: map[string]openapi.Response{
						"200": {
							Description: "User found",
							Content: map[string]openapi.MediaTypeObject{
								"application/json": {
									Schema: &openapi.Schema{
										Type: "object",
										Properties: map[string]*openapi.Schema{
											"id":    {Type: "string"},
											"name":  {Type: "string"},
											"email": {Type: "string"},
										},
										Required: []string{"id", "name", "email"},
									},
								},
							},
						},
						"404": {Description: "User not found"},
					},
				},
			},
		},
		Schemas: make(map[string]*openapi.Schema),
	}

	// Setup state management
	stateConfig := &state.Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      0,
		Storage: state.StorageConfig{
			Type: "memory",
		},
	}

	stateManager := state.NewMemoryStateManager(stateConfig)
	err := stateManager.Start()
	if err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer stateManager.Stop()

	contextManager := state.NewContextManager(&state.ContextConfig{
		DefaultTTL: 30 * time.Minute,
		SessionTTL: 24 * time.Hour,
		RequestTTL: 5 * time.Minute,
	})

	// Setup validation
	validationConfig := &validation.Config{
		Enabled:         true,
		StrictMode:      true,
		ValidateHeaders: true,
		ValidateQuery:   true,
		ValidatePath:    true,
		ValidateBody:    true,
	}

	validationManager := validation.NewValidationManager(spec, validationConfig)
	requestValidator := validationManager.GetRequestValidator()
	responseValidator := validationManager.GetResponseValidator()

	ctx := context.Background()

	t.Run("Create User with State Tracking", func(t *testing.T) {
		// Create state context for the request
		sessionID := "test-session-123"
		requestID := "req-001"
		userID := "user-001"

		stateContext := contextManager.CreateContext(sessionID, "/users", requestID, userID)
		ctxWithState := state.WithStateContext(ctx, stateContext)

		// Store initial state
		scopedStateManager := state.NewScopedStateManager(stateManager, contextManager)
		err := scopedStateManager.SetInSession(ctxWithState, "current_operation", "create_user")
		if err != nil {
			t.Fatalf("Failed to set session state: %v", err)
		}

		// Create and validate request
		userData := map[string]interface{}{
			"name":  "John Doe",
			"email": "john.doe@example.com",
		}

		jsonData, _ := json.Marshal(userData)
		req, _ := http.NewRequest("POST", "/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		// Validate request
		requestResult, err := requestValidator.ValidateRequest(ctxWithState, req)
		if err != nil {
			t.Fatalf("Request validation failed: %v", err)
		}

		if !requestResult.Valid {
			t.Fatalf("Request should be valid. Errors: %v", requestResult.Errors)
		}

		// Simulate processing and store user in state
		createdUser := map[string]interface{}{
			"id":    "user-12345",
			"name":  userData["name"],
			"email": userData["email"],
		}

		err = scopedStateManager.SetInSession(ctxWithState, "created_user", createdUser)
		if err != nil {
			t.Fatalf("Failed to store created user in state: %v", err)
		}

		// Create mock response and validate it
		mockResponse := &validation.MockResponse{
			StatusCode:  201,
			ContentType: "application/json",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: createdUser,
		}

		responseResult, err := responseValidator.ValidateMockResponse(ctxWithState, req, mockResponse)
		if err != nil {
			t.Fatalf("Response validation failed: %v", err)
		}

		if !responseResult.Valid {
			t.Fatalf("Response should be valid. Errors: %v", responseResult.Errors)
		}

		// Verify state was stored correctly
		storedOperation, err := scopedStateManager.GetFromSession(ctxWithState, "current_operation")
		if err != nil {
			t.Fatalf("Failed to retrieve operation from state: %v", err)
		}

		if storedOperation != "create_user" {
			t.Errorf("Expected operation 'create_user', got '%v'", storedOperation)
		}

		storedUser, err := scopedStateManager.GetFromSession(ctxWithState, "created_user")
		if err != nil {
			t.Fatalf("Failed to retrieve user from state: %v", err)
		}

		userMap := storedUser.(map[string]interface{})
		if userMap["id"] != "user-12345" {
			t.Errorf("Expected user ID 'user-12345', got '%v'", userMap["id"])
		}
	})

	t.Run("Get User with State Lookup", func(t *testing.T) {
		// Use same session to retrieve previously stored user
		sessionID := "test-session-123"
		requestID := "req-002"
		userID := "user-001"

		stateContext := contextManager.CreateContext(sessionID, "/users/{id}", requestID, userID)
		ctxWithState := state.WithStateContext(ctx, stateContext)

		scopedStateManager := state.NewScopedStateManager(stateManager, contextManager)

		// Retrieve previously stored user
		storedUser, err := scopedStateManager.GetFromSession(ctxWithState, "created_user")
		if err != nil {
			t.Fatalf("Failed to retrieve user from state: %v", err)
		}

		userMap := storedUser.(map[string]interface{})
		userID = userMap["id"].(string)

		// Create and validate GET request
		req, _ := http.NewRequest("GET", "/users/"+userID, nil)

		requestResult, err := requestValidator.ValidateRequest(ctxWithState, req)
		if err != nil {
			t.Fatalf("Request validation failed: %v", err)
		}

		if !requestResult.Valid {
			t.Fatalf("Request should be valid. Errors: %v", requestResult.Errors)
		}

		// Create mock response using stored user data
		mockResponse := &validation.MockResponse{
			StatusCode:  200,
			ContentType: "application/json",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: userMap,
		}

		responseResult, err := responseValidator.ValidateMockResponse(ctxWithState, req, mockResponse)
		if err != nil {
			t.Fatalf("Response validation failed: %v", err)
		}

		if !responseResult.Valid {
			t.Fatalf("Response should be valid. Errors: %v", responseResult.Errors)
		}
	})

	t.Run("Invalid Request Validation", func(t *testing.T) {
		// Test validation with invalid data
		sessionID := "test-session-456"
		requestID := "req-003"

		stateContext := contextManager.CreateContext(sessionID, "/users", requestID, "")
		ctxWithState := state.WithStateContext(ctx, stateContext)

		// Invalid user data (missing required email field)
		invalidUserData := map[string]interface{}{
			"name": "Jane Doe",
		}

		jsonData, _ := json.Marshal(invalidUserData)
		req, _ := http.NewRequest("POST", "/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")

		requestResult, err := requestValidator.ValidateRequest(ctxWithState, req)
		if err != nil {
			t.Fatalf("Request validation failed: %v", err)
		}

		if requestResult.Valid {
			t.Fatal("Request should be invalid due to missing email field")
		}

		if len(requestResult.Errors) == 0 {
			t.Fatal("Expected validation errors for invalid request")
		}

		// Store validation result in state for analysis
		scopedStateManager := state.NewScopedStateManager(stateManager, contextManager)
		err = scopedStateManager.SetInSession(ctxWithState, "validation_result", requestResult)
		if err != nil {
			t.Fatalf("Failed to store validation result: %v", err)
		}
	})

	t.Run("Coverage and Compliance Reporting", func(t *testing.T) {
		// Generate reports from validation manager
		reporter := validationManager.GetReporter()

		coverageReport := reporter.GenerateCoverageReport(spec)
		complianceReport := reporter.GenerateComplianceReport()

		// Check that endpoints were tracked
		if coverageReport.TotalEndpoints == 0 {
			t.Error("Expected some endpoints in coverage report")
		}

		// Check that requests were tracked
		if complianceReport.TotalRequests == 0 {
			t.Error("Expected some requests in compliance report")
		}

		// We should have some invalid requests from the previous test
		if complianceReport.InvalidRequests == 0 {
			t.Error("Expected some invalid requests in compliance report")
		}

		// Compliance percentage should be less than 100% due to invalid requests
		if complianceReport.CompliancePercent >= 100 {
			t.Errorf("Expected compliance < 100%%, got %.1f%%", complianceReport.CompliancePercent)
		}

		t.Logf("Coverage: %.1f%% (%d/%d endpoints)",
			coverageReport.CoveragePercent, coverageReport.CoveredEndpoints, coverageReport.TotalEndpoints)
		t.Logf("Compliance: %.1f%% (%d/%d requests)",
			complianceReport.CompliancePercent, complianceReport.ValidRequests, complianceReport.TotalRequests)
	})

	t.Run("State Export and Cleanup", func(t *testing.T) {
		// Export all state for inspection
		exportedState, err := stateManager.Export(ctx)
		if err != nil {
			t.Fatalf("Failed to export state: %v", err)
		}

		if len(exportedState) == 0 {
			t.Error("Expected some state data to be exported")
		}

		// Clean up specific session
		sessionID := "test-session-123"
		scopes := stateManager.ListScopes(ctx)

		for _, scope := range scopes {
			if strings.Contains(scope, sessionID) {
				err := stateManager.ClearScope(ctx, scope)
				if err != nil {
					t.Errorf("Failed to clear scope %s: %v", scope, err)
				}
			}
		}

		t.Logf("Exported %d state entries", len(exportedState))
		t.Logf("Found %d scopes before cleanup", len(scopes))
	})
}
