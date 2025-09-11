package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/plugins"
)

func main() {
	fmt.Println("🔧 Plugin Configuration System Demo")
	fmt.Println("=====================================")

	// Set up test environment variables
	os.Setenv("DEMO_JWT_SECRET", "demo-secret-key-that-is-very-long-for-security-purposes")
	os.Setenv("DEMO_API_KEY", "demo-api-key-123")
	defer func() {
		os.Unsetenv("DEMO_JWT_SECRET")
		os.Unsetenv("DEMO_API_KEY")
	}()

	// 1. Demonstrate configuration validation
	fmt.Println("\n📋 1. Configuration Validation")
	demonstrateValidation()

	// 2. Demonstrate environment variable substitution
	fmt.Println("\n🌍 2. Environment Variable Substitution")
	demonstrateEnvSubstitution()

	// 3. Demonstrate plugin creation
	fmt.Println("\n⚙️  3. Plugin Creation")
	demonstratePluginCreation()

	// 4. Demonstrate plugin manager integration
	fmt.Println("\n🏗️  4. Plugin Manager Integration")
	demonstrateManagerIntegration()

	// 5. Demonstrate default configurations
	fmt.Println("\n📦 5. Default Configurations")
	demonstrateDefaultConfigs()

	// 6. Demonstrate hot-reload validation
	fmt.Println("\n🔥 6. Hot-Reload Validation")
	demonstrateHotReload()

	fmt.Println("\n✅ Demo completed successfully!")
}

func demonstrateValidation() {
	// Valid configuration
	validConfig := map[string]interface{}{
		"jwt_secret": "valid-secret-key-that-is-long-enough-for-security",
		"jwt_method": "HS256",
		"api_keys": map[string]interface{}{
			"key1": "user1",
		},
	}

	err := plugins.ValidatePluginConfig("auth", validConfig)
	if err != nil {
		fmt.Printf("❌ Valid config failed validation: %v\n", err)
	} else {
		fmt.Printf("✅ Valid configuration passed validation\n")
	}

	// Invalid configuration
	invalidConfig := map[string]interface{}{
		"jwt_method": "INVALID_METHOD",
		"jwt_secret": "short", // Too short
	}

	err = plugins.ValidatePluginConfig("auth", invalidConfig)
	if err != nil {
		fmt.Printf("✅ Invalid config correctly rejected: %v\n", err)
	} else {
		fmt.Printf("❌ Invalid config incorrectly passed validation\n")
	}
}

func demonstrateEnvSubstitution() {
	config := map[string]interface{}{
		"jwt_secret": "${DEMO_JWT_SECRET}",
		"api_keys": map[string]interface{}{
			"${DEMO_API_KEY}": "demo-user",
		},
		"port":        "${PORT:8080}", // Default value
		"missing_var": "${MISSING:default_value}",
	}

	fmt.Printf("Original config: %+v\n", config)

	// Validate (this automatically applies environment substitution)
	err := plugins.ValidatePluginConfig("auth", config)
	if err != nil {
		fmt.Printf("❌ Environment substitution failed: %v\n", err)
	} else {
		fmt.Printf("✅ Environment substitution successful\n")
	}
}

func demonstratePluginCreation() {
	config := map[string]interface{}{
		"jwt_secret": "demo-secret-key-that-is-very-long-for-security-purposes",
		"jwt_method": "HS256",
		"api_keys": map[string]interface{}{
			"demo-key": "demo-user",
		},
	}

	// Create auth plugin
	plugin, err := plugins.CreatePluginFromConfig("auth", config)
	if err != nil {
		fmt.Printf("❌ Failed to create auth plugin: %v\n", err)
		return
	}

	fmt.Printf("✅ Created plugin: %s v%s - %s\n", 
		plugin.Name(), plugin.Version(), plugin.Description())

	// Cleanup
	plugin.Cleanup(context.Background())

	// Create rate limit plugin
	rateLimitConfig := map[string]interface{}{
		"ip_requests_per_second": 10.0,
		"ip_burst":              20,
	}

	plugin, err = plugins.CreatePluginFromConfig("rate_limit", rateLimitConfig)
	if err != nil {
		fmt.Printf("❌ Failed to create rate limit plugin: %v\n", err)
		return
	}

	fmt.Printf("✅ Created plugin: %s v%s - %s\n", 
		plugin.Name(), plugin.Version(), plugin.Description())

	// Cleanup
	plugin.Cleanup(context.Background())
}

func demonstrateManagerIntegration() {
	// Create plugin configurations
	pluginConfigs := []config.PluginConfig{
		{
			Name:    "auth",
			Enabled: true,
			Config: map[string]interface{}{
				"jwt_secret": "${DEMO_JWT_SECRET}",
				"jwt_method": "HS256",
				"api_keys": map[string]interface{}{
					"${DEMO_API_KEY}": "demo-user",
				},
			},
		},
		{
			Name:    "rate_limit",
			Enabled: true,
			Config: map[string]interface{}{
				"ip_requests_per_second": 10.0,
				"ip_burst":              20,
			},
		},
		{
			Name:    "cors",
			Enabled: true,
			Config: map[string]interface{}{
				"allow_origins": []interface{}{"http://localhost:3000"},
			},
		},
	}

	// Create plugin manager
	logger := zap.NewNop()
	manager := plugins.NewManager(logger)
	defer manager.Shutdown()

	// Register built-in plugins
	err := plugins.RegisterBuiltinPlugins(manager.GetRegistry())
	if err != nil {
		fmt.Printf("❌ Failed to register built-in plugins: %v\n", err)
		return
	}

	// Load plugins into manager using our configuration system
	err = plugins.LoadPluginsIntoManager(manager, pluginConfigs)
	if err != nil {
		fmt.Printf("❌ Failed to load plugins into manager: %v\n", err)
		return
	}

	// Check plugin states
	pluginInfos := manager.ListPlugins()
	fmt.Printf("✅ Loaded %d plugins into manager:\n", len(pluginInfos))
	for _, info := range pluginInfos {
		fmt.Printf("   - %s (v%s): %s\n", info.Name, info.Version, info.State)
	}

	// Get metrics
	metrics := manager.GetPluginMetrics()
	fmt.Printf("📊 Manager metrics: %d total, %d enabled\n", 
		metrics["total_plugins"], metrics["enabled_plugins"])
}

func demonstrateDefaultConfigs() {
	pluginNames := []string{"auth", "rate_limit", "cors", "logging"}

	for _, name := range pluginNames {
		defaults := plugins.GetDefaultConfig(name)
		fmt.Printf("📦 %s defaults: %d fields\n", name, len(defaults))
		
		// Show a few example defaults
		count := 0
		for key, value := range defaults {
			if count >= 2 { // Limit output
				break
			}
			fmt.Printf("   - %s: %v\n", key, value)
			count++
		}
		if len(defaults) > 2 {
			fmt.Printf("   - ... and %d more\n", len(defaults)-2)
		}
	}
}

func demonstrateHotReload() {
	oldConfig := map[string]interface{}{
		"jwt_secret": "old-secret-key-that-is-very-long-for-security-purposes",
		"jwt_method": "HS256",
	}

	// Valid hot-reload: changing secret
	newConfig := map[string]interface{}{
		"jwt_secret": "new-secret-key-that-is-very-long-for-security-purposes",
		"jwt_method": "HS256",
	}

	err := plugins.ValidateConfigForHotReload("auth", oldConfig, newConfig)
	if err != nil {
		fmt.Printf("❌ Valid hot-reload rejected: %v\n", err)
	} else {
		fmt.Printf("✅ Valid hot-reload accepted (changing JWT secret)\n")
	}

	// Invalid hot-reload: changing method
	invalidNewConfig := map[string]interface{}{
		"jwt_secret": "new-secret-key-that-is-very-long-for-security-purposes",
		"jwt_method": "RS256", // This requires restart
	}

	err = plugins.ValidateConfigForHotReload("auth", oldConfig, invalidNewConfig)
	if err != nil {
		fmt.Printf("✅ Invalid hot-reload correctly rejected: %v\n", err)
	} else {
		fmt.Printf("❌ Invalid hot-reload incorrectly accepted\n")
	}

	// Rate limit plugin supports full hot-reload
	rateLimitOld := map[string]interface{}{
		"ip_requests_per_second": 10.0,
	}

	rateLimitNew := map[string]interface{}{
		"ip_requests_per_second": 20.0,
		"ip_burst":              50,
	}

	err = plugins.ValidateConfigForHotReload("rate_limit", rateLimitOld, rateLimitNew)
	if err != nil {
		fmt.Printf("❌ Rate limit hot-reload rejected: %v\n", err)
	} else {
		fmt.Printf("✅ Rate limit hot-reload accepted (full hot-reload support)\n")
	}
}

func init() {
	// Suppress log output for cleaner demo
	log.SetOutput(os.Stderr)
}