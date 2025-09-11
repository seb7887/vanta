package plugins

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"vanta/pkg/config"
)

// TestPluginConfigurationSystemIntegration tests the complete plugin configuration system
func TestPluginConfigurationSystemIntegration(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_JWT_SECRET", "integration-test-secret-key-that-is-very-long-for-security")
	os.Setenv("TEST_API_KEY", "test-api-key-123")
	defer func() {
		os.Unsetenv("TEST_JWT_SECRET")
		os.Unsetenv("TEST_API_KEY")
	}()

	// Test complete plugin configuration workflow
	t.Run("CompleteConfigurationWorkflow", func(t *testing.T) {
		// 1. Define plugin configurations with environment variables
		pluginConfigs := []config.PluginConfig{
			{
				Name:    "auth",
				Enabled: true,
				Config: map[string]interface{}{
					"jwt_secret":       "${TEST_JWT_SECRET}",
					"jwt_method":       "HS256",
					"jwt_issuer":       "test-issuer",
					"api_keys": map[string]interface{}{
						"${TEST_API_KEY}": "test-user",
					},
					"public_endpoints": []interface{}{"/health", "/metrics"},
				},
			},
			{
				Name:    "rate_limit",
				Enabled: true,
				Config: map[string]interface{}{
					"ip_requests_per_second": 10.0,
					"ip_burst":              20,
					"exempt_ips":            []interface{}{"127.0.0.1"},
				},
			},
			{
				Name:    "cors",
				Enabled: true,
				Config: map[string]interface{}{
					"allow_origins":     []interface{}{"http://localhost:3000"},
					"allow_methods":     []interface{}{"GET", "POST"},
					"allow_credentials": false,
				},
			},
			{
				Name:    "logging",
				Enabled: true,
				Config: map[string]interface{}{
					"log_level":         "info",
					"log_request_body":  false,
					"log_response_body": false,
					"include_metrics":   true,
				},
			},
		}

		// 2. Create plugin manager
		logger := zap.NewNop()
		manager := NewManager(logger)
		defer manager.Shutdown()

		// 3. Register built-in plugins
		err := RegisterBuiltinPlugins(manager.GetRegistry())
		if err != nil {
			t.Fatalf("Failed to register built-in plugins: %v", err)
		}

		// 4. Load plugins from configuration using our system
		results, err := LoadPluginsFromConfig(pluginConfigs)
		if err != nil {
			t.Fatalf("Failed to load plugins from config: %v", err)
		}

		// 5. Verify all plugins loaded successfully
		if len(results) != len(pluginConfigs) {
			t.Errorf("Expected %d plugin results, got %d", len(pluginConfigs), len(results))
		}

		for _, result := range results {
			if result.Error != nil {
				t.Errorf("Plugin '%s' failed to load: %v", result.Name, result.Error)
				continue
			}

			if result.Plugin == nil {
				t.Errorf("Plugin '%s' is nil", result.Name)
				continue
			}

			// Load the plugin into the manager
			err := manager.LoadPlugin(result.Name, result.Config)
			if err != nil {
				t.Errorf("Failed to load plugin '%s' into manager: %v", result.Name, err)
				continue
			}

			if result.Enabled {
				err := manager.EnablePlugin(result.Name)
				if err != nil {
					t.Errorf("Failed to enable plugin '%s': %v", result.Name, err)
				}
			}
		}

		// 6. Verify plugin states
		pluginInfos := manager.ListPlugins()
		if len(pluginInfos) != len(pluginConfigs) {
			t.Errorf("Expected %d plugins in manager, got %d", len(pluginConfigs), len(pluginInfos))
		}

		for _, info := range pluginInfos {
			if info.State != StateEnabled {
				t.Errorf("Plugin '%s' should be enabled, but state is '%s'", info.Name, info.State)
			}
		}

		// 7. Test configuration validation for each plugin
		for _, pc := range pluginConfigs {
			if err := ValidatePluginConfig(pc.Name, pc.Config); err != nil {
				t.Errorf("Configuration validation failed for plugin '%s': %v", pc.Name, err)
			}
		}

		// 8. Test getting default configurations
		for _, pc := range pluginConfigs {
			defaults := GetDefaultConfig(pc.Name)
			if len(defaults) == 0 {
				t.Errorf("No default configuration found for plugin '%s'", pc.Name)
			}
		}

		// 9. Test hot-reload capability
		authConfig := map[string]interface{}{
			"jwt_secret":       "new-integration-test-secret-key-that-is-very-long-for-security",
			"jwt_method":       "HS256",
			"jwt_issuer":       "new-test-issuer",
			"api_keys": map[string]interface{}{
				"new-test-key": "new-test-user",
			},
			"public_endpoints": []interface{}{"/health", "/metrics", "/status"},
		}

		if err := ValidateConfigForHotReload("auth", pluginConfigs[0].Config, authConfig); err != nil {
			t.Errorf("Hot-reload validation failed for auth plugin: %v", err)
		}

		// 10. Test middleware creation
		middlewareFunc := manager.CreateMiddlewareFunc()
		if middlewareFunc == nil {
			t.Error("Failed to create middleware function")
		}

		// 11. Verify plugin metrics
		metrics := manager.GetPluginMetrics()
		if metrics["total_plugins"].(int) != len(pluginConfigs) {
			t.Errorf("Expected %d total plugins in metrics, got %d", len(pluginConfigs), metrics["total_plugins"])
		}

		if metrics["enabled_plugins"].(int) != len(pluginConfigs) {
			t.Errorf("Expected %d enabled plugins in metrics, got %d", len(pluginConfigs), metrics["enabled_plugins"])
		}
	})

	// Test configuration error handling
	t.Run("ConfigurationErrorHandling", func(t *testing.T) {
		// Test invalid configuration
		invalidConfig := map[string]interface{}{
			"jwt_method": "INVALID_METHOD",
		}

		err := ValidatePluginConfig("auth", invalidConfig)
		if err == nil {
			t.Error("Expected validation error for invalid configuration")
		}

		// Test missing required configuration
		emptyConfig := map[string]interface{}{}

		err = ValidatePluginConfig("auth", emptyConfig)
		if err == nil {
			t.Error("Expected validation error for empty auth configuration")
		}

		// Test plugin creation with invalid config
		_, err = CreatePluginFromConfig("auth", invalidConfig)
		if err == nil {
			t.Error("Expected error when creating plugin with invalid config")
		}
	})

	// Test environment variable substitution
	t.Run("EnvironmentVariableSubstitution", func(t *testing.T) {
		registry := NewPluginConfigRegistry()

		config := map[string]interface{}{
			"jwt_secret": "${TEST_JWT_SECRET}",
			"api_keys": map[string]interface{}{
				"${TEST_API_KEY}": "user",
			},
			"missing_var":     "${MISSING_VAR:default_value}",
			"nested": map[string]interface{}{
				"value": "${TEST_JWT_SECRET}",
			},
		}

		result := registry.substituteEnvironmentVariables(config)

		// Check that environment variables were substituted
		if result["jwt_secret"] != "integration-test-secret-key-that-is-very-long-for-security" {
			t.Errorf("Environment variable substitution failed for jwt_secret")
		}

		// Check default value handling
		if result["missing_var"] != "default_value" {
			t.Errorf("Default value handling failed for missing variable")
		}

		// Check nested substitution
		nested := result["nested"].(map[string]interface{})
		if nested["value"] != "integration-test-secret-key-that-is-very-long-for-security" {
			t.Errorf("Nested environment variable substitution failed")
		}
	})

	// Test schema-based validation
	t.Run("SchemaBasedValidation", func(t *testing.T) {
		registry := NewPluginConfigRegistry()

		// Test valid configuration
		validConfig := map[string]interface{}{
			"jwt_secret": "very-long-secret-key-for-hmac-signing-security-purposes",
			"jwt_method": "HS256",
		}

		result := registry.ValidateConfig("auth", validConfig)
		if !result.Valid {
			t.Errorf("Valid configuration failed schema validation: %+v", result.Errors)
		}

		// Test invalid types
		invalidTypeConfig := map[string]interface{}{
			"jwt_secret": 123, // Should be string
			"jwt_method": "HS256",
		}

		result = registry.ValidateConfig("auth", invalidTypeConfig)
		if result.Valid {
			t.Error("Invalid type configuration passed schema validation")
		}

		// Test missing required fields for rate limit
		incompleteRateLimitConfig := map[string]interface{}{
			"global_requests_per_second": 0.0,
			"ip_requests_per_second":     0.0,
			"user_requests_per_second":   0.0,
		}

		result = registry.ValidateConfig("rate_limit", incompleteRateLimitConfig)
		if result.Valid {
			t.Error("Incomplete rate limit configuration passed validation")
		}
	})

	// Test configuration migrations
	t.Run("ConfigurationMigrations", func(t *testing.T) {
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

		err := registry.RegisterMigration("test_plugin", migration)
		if err != nil {
			t.Fatalf("Failed to register migration: %v", err)
		}

		// Test migration
		oldConfig := map[string]interface{}{
			"old_field": "test_value",
		}

		migratedConfig, err := registry.MigrateConfig("test_plugin", oldConfig, "v1")
		if err != nil {
			t.Fatalf("Migration failed: %v", err)
		}

		if migratedConfig["new_field"] != "test_value" {
			t.Errorf("Migration failed: expected 'test_value', got '%v'", migratedConfig["new_field"])
		}

		if _, exists := migratedConfig["old_field"]; exists {
			t.Error("Old field should have been removed during migration")
		}
	})

	// Test concurrent access to configuration system
	t.Run("ConcurrentAccess", func(t *testing.T) {
		registry := NewPluginConfigRegistry()

		// Test concurrent validation
		done := make(chan bool, 10)
		config := map[string]interface{}{
			"jwt_secret": "concurrent-test-secret-key-that-is-very-long-for-security",
			"jwt_method": "HS256",
		}

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				
				// Validate configuration
				result := registry.ValidateConfig("auth", config)
				if !result.Valid {
					t.Errorf("Concurrent validation failed: %+v", result.Errors)
				}

				// Get schema
				_, exists := registry.GetSchema("auth")
				if !exists {
					t.Error("Schema not found during concurrent access")
				}

				// Substitute environment variables
				registry.substituteEnvironmentVariables(config)
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestConfigurationSystemPerformance benchmarks the configuration system
func TestConfigurationSystemPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Test performance with realistic configuration
	config := map[string]interface{}{
		"jwt_secret": "performance-test-secret-key-that-is-very-long-for-security",
		"jwt_method": "HS256",
		"jwt_issuer": "performance-test",
		"api_keys": map[string]interface{}{
			"key1": "user1",
			"key2": "user2",
			"key3": "user3",
			"key4": "user4",
			"key5": "user5",
		},
		"public_endpoints": []interface{}{
			"/health", "/metrics", "/status", "/docs", "/swagger",
		},
	}

	start := time.Now()
	iterations := 1000

	// Test validation performance
	for i := 0; i < iterations; i++ {
		err := ValidatePluginConfig("auth", config)
		if err != nil {
			t.Fatalf("Validation failed on iteration %d: %v", i, err)
		}
	}

	validationDuration := time.Since(start)
	avgValidation := validationDuration / time.Duration(iterations)

	t.Logf("Validation performance: %d iterations in %v (avg: %v per validation)", 
		iterations, validationDuration, avgValidation)

	// Test creation performance
	start = time.Now()
	for i := 0; i < 100; i++ { // Fewer iterations for creation as it's more expensive
		plugin, err := CreatePluginFromConfig("auth", config)
		if err != nil {
			t.Fatalf("Plugin creation failed on iteration %d: %v", i, err)
		}
		plugin.Cleanup(context.Background()) // Clean up
	}

	creationDuration := time.Since(start)
	avgCreation := creationDuration / 100

	t.Logf("Creation performance: 100 iterations in %v (avg: %v per creation)", 
		creationDuration, avgCreation)

	// Ensure performance is reasonable (adjust thresholds as needed)
	if avgValidation > 10*time.Millisecond {
		t.Errorf("Validation too slow: %v per validation (expected < 10ms)", avgValidation)
	}

	if avgCreation > 100*time.Millisecond {
		t.Errorf("Creation too slow: %v per creation (expected < 100ms)", avgCreation)
	}
}