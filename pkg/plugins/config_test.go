package plugins

import (
	"os"
	"testing"
	"time"
)

func TestPluginConfigRegistry(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{"RegisterSchema", testRegisterSchema},
		{"ValidateConfig", testValidateConfig},
		{"EnvironmentSubstitution", testEnvironmentSubstitution},
		{"DefaultConfig", testDefaultConfig},
		{"ConfigMigration", testConfigMigration},
		{"BuiltinPluginValidation", testBuiltinPluginValidation},
		{"HotReloadValidation", testHotReloadValidation},
		{"TypeConversion", testTypeConversion},
		{"FactoryFunctions", testFactoryFunctions},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func testRegisterSchema(t *testing.T) {
	registry := NewPluginConfigRegistry()
	
	schema := &JSONSchema{
		Type:    "object",
		Title:   "Test Plugin",
		Version: CurrentVersion,
		Properties: map[string]JSONSchemaProperty{
			"enabled": {
				Type:    "boolean",
				Default: true,
			},
		},
	}
	
	// Test registration
	err := registry.RegisterSchema("test", schema)
	if err != nil {
		t.Fatalf("Failed to register schema: %v", err)
	}
	
	// Test retrieval
	retrieved, exists := registry.GetSchema("test")
	if !exists {
		t.Fatal("Schema not found after registration")
	}
	
	if retrieved.Title != "Test Plugin" {
		t.Errorf("Expected title 'Test Plugin', got '%s'", retrieved.Title)
	}
}

func testValidateConfig(t *testing.T) {
	registry := NewPluginConfigRegistry()
	
	// Test with auth plugin (built-in schema)
	validConfig := map[string]interface{}{
		"jwt_secret": "very-secret-key-that-is-long-enough-for-security",
		"jwt_method": "HS256",
	}
	
	result := registry.ValidateConfig("auth", validConfig)
	if !result.Valid {
		t.Errorf("Valid config failed validation: %+v", result.Errors)
	}
	
	// Test invalid config
	invalidConfig := map[string]interface{}{
		"jwt_method": "INVALID_METHOD",
	}
	
	result = registry.ValidateConfig("auth", invalidConfig)
	if result.Valid {
		t.Error("Invalid config passed validation")
	}
	
	if len(result.Errors) == 0 {
		t.Error("Expected validation errors but got none")
	}
}

func testEnvironmentSubstitution(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_SECRET", "test-secret-value")
	os.Setenv("TEST_PORT", "8080")
	defer func() {
		os.Unsetenv("TEST_SECRET")
		os.Unsetenv("TEST_PORT")
	}()
	
	registry := NewPluginConfigRegistry()
	
	config := map[string]interface{}{
		"jwt_secret": "${TEST_SECRET}",
		"port":       "${TEST_PORT:3000}",
		"debug":      "${DEBUG:false}",
		"nested": map[string]interface{}{
			"value": "${TEST_SECRET}",
		},
	}
	
	result := registry.substituteEnvironmentVariables(config)
	
	if result["jwt_secret"] != "test-secret-value" {
		t.Errorf("Expected 'test-secret-value', got '%v'", result["jwt_secret"])
	}
	
	if result["port"] != "8080" {
		t.Errorf("Expected '8080', got '%v'", result["port"])
	}
	
	if result["debug"] != "false" {
		t.Errorf("Expected 'false' (default), got '%v'", result["debug"])
	}
	
	nested := result["nested"].(map[string]interface{})
	if nested["value"] != "test-secret-value" {
		t.Errorf("Expected nested value 'test-secret-value', got '%v'", nested["value"])
	}
}

func testDefaultConfig(t *testing.T) {
	// Test getting default config for auth plugin
	defaults := GetDefaultConfig("auth")
	
	// Check individual fields rather than comparing maps directly
	if defaults["jwt_method"] != "HS256" {
		t.Errorf("Expected jwt_method 'HS256', got '%v'", defaults["jwt_method"])
	}
	
	if defaults["auth_header"] != "Authorization" {
		t.Errorf("Expected auth_header 'Authorization', got '%v'", defaults["auth_header"])
	}
	
	if defaults["auth_query"] != "api_key" {
		t.Errorf("Expected auth_query 'api_key', got '%v'", defaults["auth_query"])
	}
	
	if defaults["auth_cookie"] != "auth_token" {
		t.Errorf("Expected auth_cookie 'auth_token', got '%v'", defaults["auth_cookie"])
	}
	
	// Check that api_keys and public_endpoints exist (they are empty maps/slices)
	if _, exists := defaults["api_keys"]; !exists {
		t.Error("Missing default for 'api_keys'")
	}
	
	if _, exists := defaults["public_endpoints"]; !exists {
		t.Error("Missing default for 'public_endpoints'")
	}
}

func testConfigMigration(t *testing.T) {
	registry := NewPluginConfigRegistry()
	
	// Register a test migration
	migration := ConfigMigration{
		FromVersion: "v1",
		ToVersion:   "v2",
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			// Example migration: rename 'old_field' to 'new_field'
			if oldValue, exists := config["old_field"]; exists {
				config["new_field"] = oldValue
				delete(config, "old_field")
			}
			return config, nil
		},
	}
	
	err := registry.RegisterMigration("test", migration)
	if err != nil {
		t.Fatalf("Failed to register migration: %v", err)
	}
	
	// Test migration
	oldConfig := map[string]interface{}{
		"old_field": "test_value",
	}
	
	migratedConfig, err := registry.MigrateConfig("test", oldConfig, "v1")
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}
	
	if migratedConfig["new_field"] != "test_value" {
		t.Errorf("Migration failed: expected 'test_value', got '%v'", migratedConfig["new_field"])
	}
	
	if _, exists := migratedConfig["old_field"]; exists {
		t.Error("Old field should have been removed during migration")
	}
}

func testBuiltinPluginValidation(t *testing.T) {
	testCases := []struct {
		pluginName string
		config     map[string]interface{}
		expectValid bool
		expectError string
	}{
		{
			pluginName: "auth",
			config: map[string]interface{}{
				"jwt_secret": "very-long-secret-key-for-hmac-signing",
				"jwt_method": "HS256",
			},
			expectValid: true,
		},
		{
			pluginName: "auth",
			config: map[string]interface{}{
				"jwt_method": "RS256",
				// Missing jwt_public_key
			},
			expectValid: false,
			expectError: "jwt_public_key is required",
		},
		{
			pluginName: "rate_limit",
			config: map[string]interface{}{
				"ip_requests_per_second": 10.0,
				"ip_burst":               20,
			},
			expectValid: true,
		},
		{
			pluginName: "rate_limit",
			config: map[string]interface{}{
				"global_requests_per_second": 0.0,
				"ip_requests_per_second":     0.0,
				"user_requests_per_second":   0.0,
			},
			expectValid: false,
			expectError: "at least one rate limiting method must be enabled",
		},
		{
			pluginName: "cors",
			config: map[string]interface{}{
				"allow_origins":     []interface{}{"*"},
				"allow_credentials": true,
			},
			expectValid: false,
			expectError: "cannot use wildcard origin",
		},
		{
			pluginName: "logging",
			config: map[string]interface{}{
				"log_level":         "info",
				"log_request_body":  true,
				"max_body_size":     1048576,
			},
			expectValid: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.pluginName, func(t *testing.T) {
			result := ValidatePluginConfig(tc.pluginName, tc.config)
			
			if tc.expectValid && result != nil {
				t.Errorf("Expected valid config but got error: %v", result)
			}
			
			if !tc.expectValid && result == nil {
				t.Error("Expected validation error but config was valid")
			}
			
			if !tc.expectValid && tc.expectError != "" && result != nil {
				if !contains(result.Error(), tc.expectError) {
					t.Errorf("Expected error containing '%s', got '%s'", tc.expectError, result.Error())
				}
			}
		})
	}
}

func testHotReloadValidation(t *testing.T) {
	oldConfig := map[string]interface{}{
		"jwt_secret": "old-secret-that-is-very-long-for-hmac-security",
		"jwt_method": "HS256",
	}
	
	// Valid hot-reload: changing secret
	newConfig := map[string]interface{}{
		"jwt_secret": "new-secret-that-is-very-long-for-hmac-security",
		"jwt_method": "HS256",
	}
	
	err := ValidateConfigForHotReload("auth", oldConfig, newConfig)
	if err != nil {
		t.Errorf("Expected valid hot-reload but got error: %v", err)
	}
	
	// Invalid hot-reload: changing method
	newConfigInvalid := map[string]interface{}{
		"jwt_secret": "new-secret-that-is-very-long-for-hmac-security",
		"jwt_method": "RS256",
	}
	
	err = ValidateConfigForHotReload("auth", oldConfig, newConfigInvalid)
	if err == nil {
		t.Error("Expected hot-reload validation to fail but it passed")
	}
}

func testTypeConversion(t *testing.T) {
	// Test config to struct conversion
	config := map[string]interface{}{
		"jwt_secret": "test-secret",
		"jwt_method": "HS256",
		"api_keys": map[string]interface{}{
			"key1": "user1",
			"key2": "user2",
		},
	}
	
	var authConfig AuthConfig
	err := ConvertToTypedConfig(config, &authConfig)
	if err != nil {
		t.Fatalf("Failed to convert config to struct: %v", err)
	}
	
	if authConfig.JWTSecret != "test-secret" {
		t.Errorf("Expected JWTSecret 'test-secret', got '%s'", authConfig.JWTSecret)
	}
	
	if authConfig.JWTMethod != "HS256" {
		t.Errorf("Expected JWTMethod 'HS256', got '%s'", authConfig.JWTMethod)
	}
	
	if len(authConfig.APIKeys) != 2 {
		t.Errorf("Expected 2 API keys, got %d", len(authConfig.APIKeys))
	}
	
	// Test struct to config conversion
	backToConfig, err := ConvertFromTypedConfig(authConfig)
	if err != nil {
		t.Fatalf("Failed to convert struct to config: %v", err)
	}
	
	if backToConfig["jwt_secret"] != "test-secret" {
		t.Errorf("Expected jwt_secret 'test-secret', got '%v'", backToConfig["jwt_secret"])
	}
}

func testFactoryFunctions(t *testing.T) {
	// Test CreatePluginFromConfig
	config := map[string]interface{}{
		"jwt_secret": "very-long-secret-key-for-hmac-signing-that-meets-requirements",
		"jwt_method": "HS256",
	}
	
	plugin, err := CreatePluginFromConfig("auth", config)
	if err != nil {
		t.Fatalf("Failed to create plugin from config: %v", err)
	}
	
	if plugin.Name() != "auth" {
		t.Errorf("Expected plugin name 'auth', got '%s'", plugin.Name())
	}
	
	// Test LoadPluginsFromConfig - create a simple struct for testing
	type testPluginConfig struct {
		Name    string
		Enabled bool
		Config  map[string]interface{}
	}
	
	pluginConfigs := []testPluginConfig{
		{
			Name:    "auth",
			Enabled: true,
			Config:  config,
		},
		{
			Name:    "cors",
			Enabled: true,
			Config: map[string]interface{}{
				"allow_origins": []interface{}{"http://localhost:3000"},
			},
		},
	}
	
	// Simulate LoadPluginsFromConfig behavior for testing
	var results []*PluginLoadResult
	for _, pc := range pluginConfigs {
		result := &PluginLoadResult{
			Name:     pc.Name,
			Config:   pc.Config,
			Enabled:  pc.Enabled,
			LoadedAt: time.Now(),
		}
		
		plugin, loadErr := CreatePluginFromConfig(pc.Name, pc.Config)
		if loadErr != nil {
			result.Error = loadErr
		} else {
			result.Plugin = plugin
		}
		
		results = append(results, result)
	}
	
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
	
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Plugin '%s' failed to load: %v", result.Name, result.Error)
		}
		
		if result.Plugin == nil {
			t.Errorf("Plugin '%s' is nil", result.Name)
		}
	}
}

func TestJSONSchemaValidation(t *testing.T) {
	registry := NewPluginConfigRegistry()
	
	testCases := []struct {
		name        string
		schema      JSONSchemaProperty
		value       interface{}
		expectValid bool
	}{
		{
			name: "valid string",
			schema: JSONSchemaProperty{
				Type: "string",
			},
			value:       "test",
			expectValid: true,
		},
		{
			name: "invalid string type",
			schema: JSONSchemaProperty{
				Type: "string",
			},
			value:       123,
			expectValid: false,
		},
		{
			name: "string too short",
			schema: JSONSchemaProperty{
				Type:      "string",
				MinLength: intPtr(5),
			},
			value:       "abc",
			expectValid: false,
		},
		{
			name: "string too long",
			schema: JSONSchemaProperty{
				Type:      "string",
				MaxLength: intPtr(5),
			},
			value:       "abcdefgh",
			expectValid: false,
		},
		{
			name: "string pattern match",
			schema: JSONSchemaProperty{
				Type:    "string",
				Pattern: "^[a-z]+$",
			},
			value:       "abc",
			expectValid: true,
		},
		{
			name: "string pattern mismatch",
			schema: JSONSchemaProperty{
				Type:    "string",
				Pattern: "^[a-z]+$",
			},
			value:       "ABC123",
			expectValid: false,
		},
		{
			name: "number in range",
			schema: JSONSchemaProperty{
				Type:    "number",
				Minimum: float64Ptr(0),
				Maximum: float64Ptr(100),
			},
			value:       50.5,
			expectValid: true,
		},
		{
			name: "number below minimum",
			schema: JSONSchemaProperty{
				Type:    "number",
				Minimum: float64Ptr(0),
			},
			value:       -1.0,
			expectValid: false,
		},
		{
			name: "number above maximum",
			schema: JSONSchemaProperty{
				Type:    "number",
				Maximum: float64Ptr(100),
			},
			value:       101.0,
			expectValid: false,
		},
		{
			name: "enum valid value",
			schema: JSONSchemaProperty{
				Type: "string",
				Enum: []interface{}{"red", "green", "blue"},
			},
			value:       "red",
			expectValid: true,
		},
		{
			name: "enum invalid value",
			schema: JSONSchemaProperty{
				Type: "string",
				Enum: []interface{}{"red", "green", "blue"},
			},
			value:       "yellow",
			expectValid: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errors := registry.validateProperty(tc.schema, tc.value, "test_field")
			
			hasErrors := len(errors) > 0
			if tc.expectValid && hasErrors {
				t.Errorf("Expected valid but got errors: %+v", errors)
			}
			
			if !tc.expectValid && !hasErrors {
				t.Error("Expected validation errors but got none")
			}
		})
	}
}

func TestConfigRegistryThreadSafety(t *testing.T) {
	registry := NewPluginConfigRegistry()
	
	// Test concurrent access
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			schema := &JSONSchema{
				Type:  "object",
				Title: "Test Schema " + string(rune(id)),
			}
			
			pluginName := "test_plugin_" + string(rune(id))
			
			// Register schema
			registry.RegisterSchema(pluginName, schema)
			
			// Validate config
			config := map[string]interface{}{
				"test": "value",
			}
			registry.ValidateConfig(pluginName, config)
			
			// Get schema
			registry.GetSchema(pluginName)
			
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && s[:len(substr)] == substr) || 
		   (len(s) > len(substr) && contains(s[1:], substr))
}

// Benchmark tests

func BenchmarkValidateConfig(b *testing.B) {
	config := map[string]interface{}{
		"jwt_secret": "very-long-secret-key-for-hmac-signing-benchmarking",
		"jwt_method": "HS256",
		"api_keys": map[string]interface{}{
			"key1": "user1",
			"key2": "user2",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePluginConfig("auth", config)
	}
}

func BenchmarkEnvironmentSubstitution(b *testing.B) {
	os.Setenv("BENCH_VALUE", "benchmark-value")
	defer os.Unsetenv("BENCH_VALUE")
	
	registry := NewPluginConfigRegistry()
	config := map[string]interface{}{
		"field1": "${BENCH_VALUE}",
		"field2": "${BENCH_VALUE:default}",
		"field3": "normal-value",
		"nested": map[string]interface{}{
			"field4": "${BENCH_VALUE}",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.substituteEnvironmentVariables(config)
	}
}

func BenchmarkCreatePluginFromConfig(b *testing.B) {
	config := map[string]interface{}{
		"jwt_secret": "very-long-secret-key-for-hmac-signing-benchmarking",
		"jwt_method": "HS256",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin, err := CreatePluginFromConfig("auth", config)
		if err != nil {
			b.Fatal(err)
		}
		plugin.Cleanup(nil) // Clean up
	}
}