// Example: Using Built-in Plugins Programmatically
//
// This example demonstrates how to use the built-in plugins
// programmatically without a configuration file.

package main

import (
	"fmt"
	"log"
	"time"

	"go.uber.org/zap"
	"vanta/pkg/plugins"
)

func main() {
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Create plugin manager
	manager := plugins.NewManager(logger)
	defer manager.Shutdown()

	// Register built-in plugins
	if err := plugins.RegisterBuiltinPlugins(manager.GetRegistry()); err != nil {
		log.Fatal("Failed to register built-in plugins:", err)
	}

	// Configure and load the Authentication plugin
	authConfig := map[string]interface{}{
		"jwt_secret": "my-secret-key-change-in-production",
		"jwt_method": "HS256",
		"api_keys": map[string]string{
			"dev-key-123":  "developer",
			"admin-key-456": "admin",
		},
		"public_endpoints": []string{"/health", "/metrics"},
	}

	if err := manager.LoadPlugin("auth", authConfig); err != nil {
		log.Fatal("Failed to load auth plugin:", err)
	}

	if err := manager.EnablePlugin("auth"); err != nil {
		log.Fatal("Failed to enable auth plugin:", err)
	}

	// Configure and load the Rate Limiting plugin
	rateLimitConfig := map[string]interface{}{
		"global_requests_per_second": 100.0,
		"global_burst":               200,
		"ip_requests_per_second":     10.0,
		"ip_burst":                   20,
		"exempt_ips":                 []string{"127.0.0.1", "::1"},
	}

	if err := manager.LoadPlugin("rate_limit", rateLimitConfig); err != nil {
		log.Fatal("Failed to load rate_limit plugin:", err)
	}

	if err := manager.EnablePlugin("rate_limit"); err != nil {
		log.Fatal("Failed to enable rate_limit plugin:", err)
	}

	// Configure and load the CORS plugin
	corsConfig := map[string]interface{}{
		"allow_origins": []string{
			"http://localhost:3000",
			"https://myapp.example.com",
		},
		"allow_methods": []string{
			"GET", "POST", "PUT", "DELETE", "OPTIONS",
		},
		"allow_credentials": true,
		"max_age":           3600,
		"origin_patterns": []string{
			"^https://.*\\.example\\.com$",
			"^http://localhost:[0-9]+$",
		},
	}

	if err := manager.LoadPlugin("cors", corsConfig); err != nil {
		log.Fatal("Failed to load cors plugin:", err)
	}

	if err := manager.EnablePlugin("cors"); err != nil {
		log.Fatal("Failed to enable cors plugin:", err)
	}

	// Configure and load the Logging plugin
	loggingConfig := map[string]interface{}{
		"log_level":          "info",
		"log_request_body":   true,
		"log_response_body":  false,
		"max_body_size":      4096,
		"sensitive_headers":  []string{"authorization", "x-secret"},
		"sensitive_fields":   []string{"password", "secret", "token"},
		"include_metrics":    true,
	}

	if err := manager.LoadPlugin("logging", loggingConfig); err != nil {
		log.Fatal("Failed to load logging plugin:", err)
	}

	if err := manager.EnablePlugin("logging"); err != nil {
		log.Fatal("Failed to enable logging plugin:", err)
	}

	// Display loaded plugins
	fmt.Println("Loaded Plugins:")
	fmt.Println("===============")
	
	pluginInfos := manager.ListPlugins()
	for _, info := range pluginInfos {
		fmt.Printf("Plugin: %s\n", info.Name)
		fmt.Printf("  Version: %s\n", info.Version)
		fmt.Printf("  Description: %s\n", info.Description)
		fmt.Printf("  State: %s\n", info.State)
		fmt.Printf("  Loaded At: %s\n", info.LoadedAt.Format(time.RFC3339))
		
		if info.Health != nil {
			fmt.Printf("  Health: %s", map[bool]string{true: "Healthy", false: "Unhealthy"}[info.Health.Healthy])
			if !info.Health.Healthy {
				fmt.Printf(" (%s)", info.Health.Message)
			}
			fmt.Println()
		}
		
		fmt.Printf("  Requests Processed: %d\n", info.Metrics.RequestsProcessed)
		fmt.Printf("  Error Count: %d\n", info.Metrics.ErrorCount)
		if info.Metrics.RequestsProcessed > 0 {
			fmt.Printf("  Average Latency: %v\n", info.Metrics.AverageLatency)
		}
		fmt.Println()
	}

	// Get middleware function for use with FastHTTP
	_ = manager.CreateMiddlewareFunc()
	
	fmt.Println("Middleware chain created successfully!")
	fmt.Println("You can now use the middleware function with your FastHTTP server.")
	fmt.Println()

	// Example of creating individual plugins
	fmt.Println("Creating individual plugin instances:")
	fmt.Println("=====================================")

	// Create individual plugins
	authPlugin, err := plugins.CreateBuiltinPlugin("auth")
	if err != nil {
		log.Fatal("Failed to create auth plugin:", err)
	}
	fmt.Printf("Created: %s v%s - %s\n", authPlugin.Name(), authPlugin.Version(), authPlugin.Description())

	rateLimitPlugin, err := plugins.CreateBuiltinPlugin("rate_limit")
	if err != nil {
		log.Fatal("Failed to create rate_limit plugin:", err)
	}
	fmt.Printf("Created: %s v%s - %s\n", rateLimitPlugin.Name(), rateLimitPlugin.Version(), rateLimitPlugin.Description())

	corsPlugin, err := plugins.CreateBuiltinPlugin("cors")
	if err != nil {
		log.Fatal("Failed to create cors plugin:", err)
	}
	fmt.Printf("Created: %s v%s - %s\n", corsPlugin.Name(), corsPlugin.Version(), corsPlugin.Description())

	loggingPlugin, err := plugins.CreateBuiltinPlugin("logging")
	if err != nil {
		log.Fatal("Failed to create logging plugin:", err)
	}
	fmt.Printf("Created: %s v%s - %s\n", loggingPlugin.Name(), loggingPlugin.Version(), loggingPlugin.Description())

	fmt.Println()
	fmt.Println("Available built-in plugins:", plugins.GetBuiltinPluginNames())

	// Demonstrate plugin metrics
	fmt.Println("\nPlugin Manager Metrics:")
	fmt.Println("=======================")
	metrics := manager.GetPluginMetrics()
	for key, value := range metrics {
		if key != "plugin_stats" {
			fmt.Printf("%s: %v\n", key, value)
		}
	}

	fmt.Println("\nExample completed successfully!")
	fmt.Println("The built-in plugins are ready for production use.")
}