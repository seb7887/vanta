package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
	"vanta/pkg/plugins"
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Create plugin manager
	manager := plugins.NewManager(logger)
	defer manager.Shutdown()

	// Register built-in plugins
	if err := plugins.RegisterBuiltinPlugins(manager.GetRegistry()); err != nil {
		logger.Fatal("Failed to register built-in plugins", zap.Error(err))
	}

	// Load and configure plugins
	pluginConfigs := []config.PluginConfig{
		{
			Name:    "auth",
			Enabled: true,
			Config: map[string]interface{}{
				"jwt_secret":   "demo-secret-key-change-in-production",
				"jwt_method":   "HS256",
				"jwt_issuer":   "vanta-demo",
				"jwt_audience": "demo-users",
				"api_keys": map[string]string{
					"demo-admin-key": "admin",
					"demo-user-key":  "user",
				},
				"public_endpoints": []string{"/health", "/demo"},
			},
		},
		{
			Name:    "rate_limit",
			Enabled: true,
			Config: map[string]interface{}{
				"global_requests_per_second": 50.0,
				"global_burst":               100,
				"ip_requests_per_second":     10.0,
				"ip_burst":                   20,
				"exempt_ips":                 []string{"127.0.0.1"},
			},
		},
		{
			Name:    "cors",
			Enabled: true,
			Config: map[string]interface{}{
				"allow_origins":     []string{"http://localhost:3000", "*"},
				"allow_methods":     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				"allow_headers":     []string{"Origin", "Content-Type", "Accept", "Authorization"},
				"allow_credentials": true,
				"max_age":           3600,
			},
		},
		{
			Name:    "logging",
			Enabled: true,
			Config: map[string]interface{}{
				"log_level":         "info",
				"log_request_body":  true,
				"log_response_body": true,
				"max_body_size":     1024,
				"include_metrics":   true,
			},
		},
	}

	// Load plugins
	if err := manager.LoadFromConfig(pluginConfigs); err != nil {
		logger.Fatal("Failed to load plugins", zap.Error(err))
	}

	// Create HTTP server with plugin middleware
	middlewareFunc := manager.CreateMiddlewareFunc()

	// Create main handler
	mainHandler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		switch path {
		case "/health":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/json")
			ctx.SetBody([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))

		case "/demo":
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/json")
			ctx.SetBody([]byte(`{"message":"Hello from Vanta OpenAPI Mocker!","plugins":"All built-in plugins are active"}`))

		case "/protected":
			// This endpoint requires authentication
			userID := ctx.UserValue("user_id")
			authMethod := ctx.UserValue("auth_method")
			
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/json")
			response := fmt.Sprintf(`{
				"message":"Access granted to protected resource",
				"user_id":"%v",
				"auth_method":"%v",
				"timestamp":"%s"
			}`, userID, authMethod, time.Now().Format(time.RFC3339))
			ctx.SetBody([]byte(response))

		case "/api/users":
			// Simulate a JSON API endpoint
			if method == "GET" {
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetContentType("application/json")
				ctx.SetBody([]byte(`[
					{"id":1,"name":"John Doe","email":"john@example.com"},
					{"id":2,"name":"Jane Smith","email":"jane@example.com"}
				]`))
			} else if method == "POST" {
				ctx.SetStatusCode(fasthttp.StatusCreated)
				ctx.SetContentType("application/json")
				ctx.SetBody([]byte(`{"id":3,"name":"New User","email":"new@example.com","created":"` + time.Now().Format(time.RFC3339) + `"}`))
			} else {
				ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
				ctx.SetBody([]byte(`{"error":"method_not_allowed"}`))
			}

		case "/api/slow":
			// Simulate a slow endpoint for rate limiting demo
			time.Sleep(100 * time.Millisecond)
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/json")
			ctx.SetBody([]byte(`{"message":"This endpoint is intentionally slow for rate limiting demo"}`))

		default:
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			ctx.SetContentType("application/json")
			ctx.SetBody([]byte(`{"error":"not_found","message":"Endpoint not found"}`))
		}
	}

	// Wrap main handler with plugin middleware
	handler := middlewareFunc(mainHandler)

	// Create and configure server
	server := &fasthttp.Server{
		Handler:        handler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxConnsPerIP:  1000,
		MaxRequestsPerConn: 1000,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", zap.String("addr", ":8080"))
		if err := server.ListenAndServe(":8080"); err != nil {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Print usage information
	logger.Info("Built-in plugins demo server started")
	fmt.Println("\n=== Vanta Built-in Plugins Demo ===")
	fmt.Println("Server running on http://localhost:8080")
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("  GET  /health        - Health check (public)")
	fmt.Println("  GET  /demo          - Demo endpoint (public)")
	fmt.Println("  GET  /protected     - Protected endpoint (requires auth)")
	fmt.Println("  GET  /api/users     - List users (requires auth)")
	fmt.Println("  POST /api/users     - Create user (requires auth)")
	fmt.Println("  GET  /api/slow      - Slow endpoint for rate limit testing (requires auth)")
	fmt.Println("\nAuthentication:")
	fmt.Println("  API Keys:")
	fmt.Println("    - demo-admin-key (admin user)")
	fmt.Println("    - demo-user-key  (regular user)")
	fmt.Println("  Usage: Add 'Authorization: demo-admin-key' header")
	fmt.Println("\nTesting examples:")
	fmt.Println("  # Public endpoint")
	fmt.Println("  curl http://localhost:8080/health")
	fmt.Println()
	fmt.Println("  # Protected endpoint with API key")
	fmt.Println("  curl -H 'Authorization: demo-admin-key' http://localhost:8080/protected")
	fmt.Println()
	fmt.Println("  # CORS preflight request")
	fmt.Println("  curl -X OPTIONS -H 'Origin: http://localhost:3000' \\")
	fmt.Println("       -H 'Access-Control-Request-Method: POST' \\")
	fmt.Println("       http://localhost:8080/api/users")
	fmt.Println()
	fmt.Println("  # Rate limiting test (run multiple times quickly)")
	fmt.Println("  for i in {1..15}; do curl http://localhost:8080/api/slow & done")
	fmt.Println()
	fmt.Println("Active plugins:")
	for _, info := range manager.ListPlugins() {
		status := "disabled"
		if info.State == plugins.StateEnabled {
			status = "enabled"
		}
		fmt.Printf("  - %s v%s (%s): %s\n", info.Name, info.Version, status, info.Description)
	}
	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.ShutdownWithContext(ctx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}