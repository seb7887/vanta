// Package plugins provides built-in plugins for the OpenAPI mocker.
// These production-ready plugins demonstrate the plugin system capabilities
// while providing essential functionality for API mocking scenarios.
package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

// Version constants for built-in plugins
const (
	BuiltinVersion = "1.0.0"
)

// Common error responses
var (
	unauthorizedResponse = []byte(`{"error":"unauthorized","message":"Authentication required"}`)
	forbiddenResponse    = []byte(`{"error":"forbidden","message":"Access denied"}`)
	rateLimitResponse    = []byte(`{"error":"rate_limit_exceeded","message":"Too many requests"}`)
	corsErrorResponse    = []byte(`{"error":"cors_error","message":"CORS policy violation"}`)
)

// =============================================================================
// AUTH PLUGIN - JWT and API Key Authentication
// =============================================================================

// AuthPlugin provides JWT and API key authentication functionality
type AuthPlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	
	// Configuration
	jwtSecret       []byte
	jwtPublicKey    interface{}
	apiKeys         map[string]string // key -> user_id
	publicEndpoints map[string]bool   // path patterns that don't require auth
	authHeader      string            // header name for API key auth
	authQuery       string            // query param name for API key auth
	authCookie      string            // cookie name for API key auth
	
	// JWT configuration
	jwtSigningMethod jwt.SigningMethod
	jwtIssuer        string
	jwtAudience      string
	
	mu sync.RWMutex
}

// AuthConfig defines configuration for the AuthPlugin
type AuthConfig struct {
	// JWT configuration
	JWTSecret       string `json:"jwt_secret" yaml:"jwt_secret"`
	JWTPublicKey    string `json:"jwt_public_key" yaml:"jwt_public_key"`
	JWTMethod       string `json:"jwt_method" yaml:"jwt_method"`           // HS256, RS256, etc.
	JWTIssuer       string `json:"jwt_issuer" yaml:"jwt_issuer"`
	JWTAudience     string `json:"jwt_audience" yaml:"jwt_audience"`
	
	// API Key configuration
	APIKeys         map[string]string `json:"api_keys" yaml:"api_keys"`   // key -> user_id
	AuthHeader      string            `json:"auth_header" yaml:"auth_header"`
	AuthQuery       string            `json:"auth_query" yaml:"auth_query"`
	AuthCookie      string            `json:"auth_cookie" yaml:"auth_cookie"`
	
	// Public endpoints (no auth required)
	PublicEndpoints []string `json:"public_endpoints" yaml:"public_endpoints"`
}

// NewAuthPlugin creates a new AuthPlugin instance
func NewAuthPlugin() Plugin {
	return &AuthPlugin{
		name:            "auth",
		version:         BuiltinVersion,
		description:     "JWT and API key authentication plugin",
		apiKeys:         make(map[string]string),
		publicEndpoints: make(map[string]bool),
		authHeader:      "Authorization",
		authQuery:       "api_key",
		authCookie:      "auth_token",
	}
}

func (p *AuthPlugin) Name() string        { return p.name }
func (p *AuthPlugin) Version() string     { return p.version }
func (p *AuthPlugin) Description() string { return p.description }

func (p *AuthPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger.With(zap.String("plugin", p.name))
	
	// Parse configuration
	var authConfig AuthConfig
	if err := mapToStruct(config, &authConfig); err != nil {
		return fmt.Errorf("invalid auth config: %w", err)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Configure JWT
	if authConfig.JWTSecret != "" {
		p.jwtSecret = []byte(authConfig.JWTSecret)
	}
	
	if authConfig.JWTMethod != "" {
		switch authConfig.JWTMethod {
		case "HS256":
			p.jwtSigningMethod = jwt.SigningMethodHS256
		case "HS384":
			p.jwtSigningMethod = jwt.SigningMethodHS384
		case "HS512":
			p.jwtSigningMethod = jwt.SigningMethodHS512
		case "RS256":
			p.jwtSigningMethod = jwt.SigningMethodRS256
		case "RS384":
			p.jwtSigningMethod = jwt.SigningMethodRS384
		case "RS512":
			p.jwtSigningMethod = jwt.SigningMethodRS512
		default:
			return fmt.Errorf("unsupported JWT method: %s", authConfig.JWTMethod)
		}
	} else {
		p.jwtSigningMethod = jwt.SigningMethodHS256
	}
	
	p.jwtIssuer = authConfig.JWTIssuer
	p.jwtAudience = authConfig.JWTAudience
	
	// Configure API keys
	if authConfig.APIKeys != nil {
		for key, userID := range authConfig.APIKeys {
			p.apiKeys[key] = userID
		}
	}
	
	// Configure auth sources
	if authConfig.AuthHeader != "" {
		p.authHeader = authConfig.AuthHeader
	}
	if authConfig.AuthQuery != "" {
		p.authQuery = authConfig.AuthQuery
	}
	if authConfig.AuthCookie != "" {
		p.authCookie = authConfig.AuthCookie
	}
	
	// Configure public endpoints
	for _, endpoint := range authConfig.PublicEndpoints {
		p.publicEndpoints[endpoint] = true
	}
	
	p.logger.Info("Auth plugin initialized",
		zap.Int("api_keys", len(p.apiKeys)),
		zap.Int("public_endpoints", len(p.publicEndpoints)),
		zap.String("jwt_method", authConfig.JWTMethod))
	
	return nil
}

func (p *AuthPlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("Auth plugin cleanup completed")
	return nil
}

func (p *AuthPlugin) Priority() Priority {
	return PriorityHigh // Authentication should run first
}

func (p *AuthPlugin) PreProcess(ctx *RequestContext) (bool, error) {
	path := ctx.Path()
	
	// Check if endpoint is public
	p.mu.RLock()
	isPublic := p.publicEndpoints[path]
	p.mu.RUnlock()
	
	if isPublic {
		return true, nil
	}
	
	// Try JWT authentication first
	if token := p.extractJWT(ctx); token != "" {
		if userID, err := p.validateJWT(token); err == nil {
			ctx.SetUserValue("user_id", userID)
			ctx.SetUserValue("auth_method", "jwt")
			return true, nil
		}
	}
	
	// Try API key authentication
	if apiKey := p.extractAPIKey(ctx); apiKey != "" {
		if userID, valid := p.validateAPIKey(apiKey); valid {
			ctx.SetUserValue("user_id", userID)
			ctx.SetUserValue("auth_method", "api_key")
			return true, nil
		}
	}
	
	// Authentication failed
	ctx.RequestCtx.SetStatusCode(fasthttp.StatusUnauthorized)
	ctx.RequestCtx.SetContentType("application/json")
	ctx.RequestCtx.SetBody(unauthorizedResponse)
	
	p.logger.Warn("Authentication failed",
		zap.String("path", path),
		zap.String("method", ctx.Method()),
		zap.String("remote_addr", ctx.RemoteAddr()))
	
	return false, nil
}

func (p *AuthPlugin) PostProcess(ctx *ResponseContext) error {
	// No post-processing needed for auth
	return nil
}

func (p *AuthPlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	// Apply to all requests
	return true
}

func (p *AuthPlugin) extractJWT(ctx *RequestContext) string {
	// Check Authorization header (Bearer token)
	authHeader := ctx.Header("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	
	// Check cookie
	return string(ctx.RequestCtx.Request.Header.Cookie(p.authCookie))
}

func (p *AuthPlugin) extractAPIKey(ctx *RequestContext) string {
	// Check header
	if key := ctx.Header(p.authHeader); key != "" && !strings.HasPrefix(key, "Bearer ") {
		return key
	}
	
	// Check query parameter
	if key := ctx.Query(p.authQuery); key != "" {
		return key
	}
	
	// Check cookie
	return string(ctx.RequestCtx.Request.Header.Cookie(p.authCookie))
}

func (p *AuthPlugin) validateJWT(tokenString string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if token.Method != p.jwtSigningMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		// Return the appropriate key based on signing method
		switch p.jwtSigningMethod {
		case jwt.SigningMethodHS256, jwt.SigningMethodHS384, jwt.SigningMethodHS512:
			return p.jwtSecret, nil
		default:
			return p.jwtPublicKey, nil
		}
	})
	
	if err != nil {
		return "", err
	}
	
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	
	// Verify issuer if configured
	if p.jwtIssuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != p.jwtIssuer {
			return "", fmt.Errorf("invalid issuer")
		}
	}
	
	// Verify audience if configured
	if p.jwtAudience != "" {
		if aud, ok := claims["aud"].(string); !ok || aud != p.jwtAudience {
			return "", fmt.Errorf("invalid audience")
		}
	}
	
	// Extract user ID
	userID, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("missing subject claim")
	}
	
	return userID, nil
}

func (p *AuthPlugin) validateAPIKey(apiKey string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	userID, exists := p.apiKeys[apiKey]
	return userID, exists
}

// =============================================================================
// RATE LIMIT PLUGIN - Sliding Window Rate Limiting
// =============================================================================

// RateLimitPlugin provides sliding window rate limiting
type RateLimitPlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	
	// Rate limiters
	globalLimiter *rate.Limiter
	ipLimiters    map[string]*rateLimiterEntry
	userLimiters  map[string]*rateLimiterEntry
	
	// Configuration
	globalLimit   rate.Limit
	globalBurst   int
	ipLimit       rate.Limit
	ipBurst       int
	userLimit     rate.Limit
	userBurst     int
	exemptIPs     map[string]bool
	
	// Cleanup
	cleanupInterval time.Duration
	entryTTL        time.Duration
	
	mu sync.RWMutex
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// RateLimitConfig defines configuration for the RateLimitPlugin
type RateLimitConfig struct {
	// Global rate limiting
	GlobalRequestsPerSecond float64 `json:"global_requests_per_second" yaml:"global_requests_per_second"`
	GlobalBurst             int     `json:"global_burst" yaml:"global_burst"`
	
	// Per-IP rate limiting
	IPRequestsPerSecond float64 `json:"ip_requests_per_second" yaml:"ip_requests_per_second"`
	IPBurst             int     `json:"ip_burst" yaml:"ip_burst"`
	
	// Per-user rate limiting
	UserRequestsPerSecond float64 `json:"user_requests_per_second" yaml:"user_requests_per_second"`
	UserBurst             int     `json:"user_burst" yaml:"user_burst"`
	
	// Exempt IPs (no rate limiting)
	ExemptIPs []string `json:"exempt_ips" yaml:"exempt_ips"`
	
	// Cleanup configuration
	CleanupIntervalSeconds int `json:"cleanup_interval_seconds" yaml:"cleanup_interval_seconds"`
	EntryTTLSeconds        int `json:"entry_ttl_seconds" yaml:"entry_ttl_seconds"`
}

// NewRateLimitPlugin creates a new RateLimitPlugin instance
func NewRateLimitPlugin() Plugin {
	return &RateLimitPlugin{
		name:            "rate_limit",
		version:         BuiltinVersion,
		description:     "Sliding window rate limiting plugin",
		ipLimiters:      make(map[string]*rateLimiterEntry),
		userLimiters:    make(map[string]*rateLimiterEntry),
		exemptIPs:       make(map[string]bool),
		cleanupInterval: 5 * time.Minute,
		entryTTL:        30 * time.Minute,
	}
}

func (p *RateLimitPlugin) Name() string        { return p.name }
func (p *RateLimitPlugin) Version() string     { return p.version }
func (p *RateLimitPlugin) Description() string { return p.description }

func (p *RateLimitPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger.With(zap.String("plugin", p.name))
	
	// Parse configuration
	var rlConfig RateLimitConfig
	if err := mapToStruct(config, &rlConfig); err != nil {
		return fmt.Errorf("invalid rate limit config: %w", err)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Configure global rate limiting
	if rlConfig.GlobalRequestsPerSecond > 0 {
		p.globalLimit = rate.Limit(rlConfig.GlobalRequestsPerSecond)
		p.globalBurst = rlConfig.GlobalBurst
		if p.globalBurst <= 0 {
			p.globalBurst = int(rlConfig.GlobalRequestsPerSecond)
		}
		p.globalLimiter = rate.NewLimiter(p.globalLimit, p.globalBurst)
	}
	
	// Configure IP rate limiting
	if rlConfig.IPRequestsPerSecond > 0 {
		p.ipLimit = rate.Limit(rlConfig.IPRequestsPerSecond)
		p.ipBurst = rlConfig.IPBurst
		if p.ipBurst <= 0 {
			p.ipBurst = int(rlConfig.IPRequestsPerSecond)
		}
	}
	
	// Configure user rate limiting
	if rlConfig.UserRequestsPerSecond > 0 {
		p.userLimit = rate.Limit(rlConfig.UserRequestsPerSecond)
		p.userBurst = rlConfig.UserBurst
		if p.userBurst <= 0 {
			p.userBurst = int(rlConfig.UserRequestsPerSecond)
		}
	}
	
	// Configure exempt IPs
	for _, ip := range rlConfig.ExemptIPs {
		p.exemptIPs[ip] = true
	}
	
	// Configure cleanup
	if rlConfig.CleanupIntervalSeconds > 0 {
		p.cleanupInterval = time.Duration(rlConfig.CleanupIntervalSeconds) * time.Second
	}
	if rlConfig.EntryTTLSeconds > 0 {
		p.entryTTL = time.Duration(rlConfig.EntryTTLSeconds) * time.Second
	}
	
	// Start cleanup goroutine
	go p.cleanupLoop(ctx)
	
	p.logger.Info("Rate limit plugin initialized",
		zap.Float64("global_limit", float64(p.globalLimit)),
		zap.Int("global_burst", p.globalBurst),
		zap.Float64("ip_limit", float64(p.ipLimit)),
		zap.Int("ip_burst", p.ipBurst),
		zap.Float64("user_limit", float64(p.userLimit)),
		zap.Int("user_burst", p.userBurst),
		zap.Int("exempt_ips", len(p.exemptIPs)))
	
	return nil
}

func (p *RateLimitPlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("Rate limit plugin cleanup completed")
	return nil
}

func (p *RateLimitPlugin) Priority() Priority {
	return PriorityNormal
}

func (p *RateLimitPlugin) PreProcess(ctx *RequestContext) (bool, error) {
	clientIP := p.getClientIP(ctx)
	
	// Check if IP is exempt
	p.mu.RLock()
	exempt := p.exemptIPs[clientIP]
	p.mu.RUnlock()
	
	if exempt {
		return true, nil
	}
	
	// Check global rate limit
	if p.globalLimiter != nil && !p.globalLimiter.Allow() {
		return p.rateLimitExceeded(ctx, "global")
	}
	
	// Check IP rate limit
	if p.ipLimit > 0 {
		ipLimiter := p.getIPLimiter(clientIP)
		if !ipLimiter.Allow() {
			return p.rateLimitExceeded(ctx, "ip")
		}
	}
	
	// Check user rate limit (if authenticated)
	if userID, exists := ctx.GetUserValue("user_id"); exists && p.userLimit > 0 {
		if userIDStr, ok := userID.(string); ok {
			userLimiter := p.getUserLimiter(userIDStr)
			if !userLimiter.Allow() {
				return p.rateLimitExceeded(ctx, "user")
			}
		}
	}
	
	return true, nil
}

func (p *RateLimitPlugin) PostProcess(ctx *ResponseContext) error {
	// Add rate limit headers
	clientIP := p.getClientIP(ctx.RequestContext)
	
	if p.ipLimit > 0 {
		ipLimiter := p.getIPLimiter(clientIP)
		tokens := ipLimiter.Tokens()
		
		ctx.RequestCtx.Response.Header.Set("X-RateLimit-Limit", strconv.Itoa(p.ipBurst))
		ctx.RequestCtx.Response.Header.Set("X-RateLimit-Remaining", strconv.Itoa(int(tokens)))
		ctx.RequestCtx.Response.Header.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
	}
	
	return nil
}

func (p *RateLimitPlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	return true
}

func (p *RateLimitPlugin) getClientIP(ctx *RequestContext) string {
	// Check X-Forwarded-For header
	if xff := ctx.Header("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := ctx.Header("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Use remote address
	host, _, err := net.SplitHostPort(ctx.RemoteAddr())
	if err != nil {
		return ctx.RemoteAddr()
	}
	
	return host
}

func (p *RateLimitPlugin) getIPLimiter(ip string) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	entry, exists := p.ipLimiters[ip]
	if !exists || time.Since(entry.lastUsed) > p.entryTTL {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(p.ipLimit, p.ipBurst),
			lastUsed: time.Now(),
		}
		p.ipLimiters[ip] = entry
	} else {
		entry.lastUsed = time.Now()
	}
	
	return entry.limiter
}

func (p *RateLimitPlugin) getUserLimiter(userID string) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	entry, exists := p.userLimiters[userID]
	if !exists || time.Since(entry.lastUsed) > p.entryTTL {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(p.userLimit, p.userBurst),
			lastUsed: time.Now(),
		}
		p.userLimiters[userID] = entry
	} else {
		entry.lastUsed = time.Now()
	}
	
	return entry.limiter
}

func (p *RateLimitPlugin) rateLimitExceeded(ctx *RequestContext, limitType string) (bool, error) {
	ctx.RequestCtx.SetStatusCode(fasthttp.StatusTooManyRequests)
	ctx.RequestCtx.SetContentType("application/json")
	ctx.RequestCtx.SetBody(rateLimitResponse)
	
	// Add Retry-After header
	ctx.RequestCtx.Response.Header.Set("Retry-After", "1")
	
	p.logger.Warn("Rate limit exceeded",
		zap.String("limit_type", limitType),
		zap.String("client_ip", p.getClientIP(ctx)),
		zap.String("path", ctx.Path()))
	
	return false, nil
}

func (p *RateLimitPlugin) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *RateLimitPlugin) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	now := time.Now()
	
	// Cleanup IP limiters
	for ip, entry := range p.ipLimiters {
		if now.Sub(entry.lastUsed) > p.entryTTL {
			delete(p.ipLimiters, ip)
		}
	}
	
	// Cleanup user limiters
	for userID, entry := range p.userLimiters {
		if now.Sub(entry.lastUsed) > p.entryTTL {
			delete(p.userLimiters, userID)
		}
	}
	
	p.logger.Debug("Rate limiter cleanup completed",
		zap.Int("ip_limiters", len(p.ipLimiters)),
		zap.Int("user_limiters", len(p.userLimiters)))
}

// =============================================================================
// CORS PLUGIN - Enhanced CORS Management
// =============================================================================

// CORSPlugin provides enhanced CORS functionality
type CORSPlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	
	// Configuration
	allowOrigins     []string
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	allowCredentials bool
	maxAge           int
	
	// Dynamic origin validation
	originPatterns []*regexp.Regexp
	originValidator func(string) bool
	
	mu sync.RWMutex
}

// CORSConfig defines configuration for the CORSPlugin
type CORSConfig struct {
	AllowOrigins     []string `json:"allow_origins" yaml:"allow_origins"`
	AllowMethods     []string `json:"allow_methods" yaml:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers" yaml:"allow_headers"`
	ExposeHeaders    []string `json:"expose_headers" yaml:"expose_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age"`
	OriginPatterns   []string `json:"origin_patterns" yaml:"origin_patterns"`
}

// NewCORSPlugin creates a new CORSPlugin instance
func NewCORSPlugin() Plugin {
	return &CORSPlugin{
		name:        "cors",
		version:     BuiltinVersion,
		description: "Enhanced CORS management plugin",
		allowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
		allowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		maxAge:       86400, // 24 hours
	}
}

func (p *CORSPlugin) Name() string        { return p.name }
func (p *CORSPlugin) Version() string     { return p.version }
func (p *CORSPlugin) Description() string { return p.description }

func (p *CORSPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger.With(zap.String("plugin", p.name))
	
	// Parse configuration
	var corsConfig CORSConfig
	if err := mapToStruct(config, &corsConfig); err != nil {
		return fmt.Errorf("invalid CORS config: %w", err)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Configure allowed origins
	if len(corsConfig.AllowOrigins) > 0 {
		p.allowOrigins = corsConfig.AllowOrigins
	}
	
	// Configure allowed methods
	if len(corsConfig.AllowMethods) > 0 {
		p.allowMethods = corsConfig.AllowMethods
	}
	
	// Configure allowed headers
	if len(corsConfig.AllowHeaders) > 0 {
		p.allowHeaders = corsConfig.AllowHeaders
	}
	
	// Configure exposed headers
	if len(corsConfig.ExposeHeaders) > 0 {
		p.exposeHeaders = corsConfig.ExposeHeaders
	}
	
	// Configure credentials
	p.allowCredentials = corsConfig.AllowCredentials
	
	// Configure max age
	if corsConfig.MaxAge > 0 {
		p.maxAge = corsConfig.MaxAge
	}
	
	// Compile origin patterns
	for _, pattern := range corsConfig.OriginPatterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			p.originPatterns = append(p.originPatterns, regex)
		} else {
			p.logger.Warn("Invalid origin pattern", zap.String("pattern", pattern), zap.Error(err))
		}
	}
	
	p.logger.Info("CORS plugin initialized",
		zap.Strings("allow_origins", p.allowOrigins),
		zap.Strings("allow_methods", p.allowMethods),
		zap.Bool("allow_credentials", p.allowCredentials),
		zap.Int("max_age", p.maxAge))
	
	return nil
}

func (p *CORSPlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("CORS plugin cleanup completed")
	return nil
}

func (p *CORSPlugin) Priority() Priority {
	return PriorityNormal
}

func (p *CORSPlugin) PreProcess(ctx *RequestContext) (bool, error) {
	origin := ctx.Header("Origin")
	
	// Handle preflight requests
	if ctx.Method() == "OPTIONS" {
		return p.handlePreflight(ctx, origin)
	}
	
	// Handle simple requests
	if origin != "" {
		if p.isOriginAllowed(origin) {
			p.setCORSHeaders(ctx, origin, false)
		} else {
			return p.corsError(ctx, "Origin not allowed")
		}
	}
	
	return true, nil
}

func (p *CORSPlugin) PostProcess(ctx *ResponseContext) error {
	// CORS headers are set in PreProcess
	return nil
}

func (p *CORSPlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	// Apply when there's an Origin header or it's a preflight request
	origin := string(req.Request.Header.Peek("Origin"))
	method := string(req.Method())
	return origin != "" || method == "OPTIONS"
}

func (p *CORSPlugin) handlePreflight(ctx *RequestContext, origin string) (bool, error) {
	if !p.isOriginAllowed(origin) {
		return p.corsError(ctx, "Origin not allowed for preflight")
	}
	
	// Check requested method
	requestedMethod := ctx.Header("Access-Control-Request-Method")
	if requestedMethod != "" && !p.isMethodAllowed(requestedMethod) {
		return p.corsError(ctx, "Method not allowed")
	}
	
	// Check requested headers
	requestedHeaders := ctx.Header("Access-Control-Request-Headers")
	if requestedHeaders != "" && !p.areHeadersAllowed(requestedHeaders) {
		return p.corsError(ctx, "Headers not allowed")
	}
	
	// Set preflight headers
	p.setCORSHeaders(ctx, origin, true)
	
	// Return 204 No Content for preflight
	ctx.RequestCtx.SetStatusCode(fasthttp.StatusNoContent)
	
	return false, nil // Stop processing for preflight
}

func (p *CORSPlugin) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Check explicit origins
	for _, allowedOrigin := range p.allowOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}
	
	// Check origin patterns
	for _, pattern := range p.originPatterns {
		if pattern.MatchString(origin) {
			return true
		}
	}
	
	// Check custom validator
	if p.originValidator != nil {
		return p.originValidator(origin)
	}
	
	return false
}

func (p *CORSPlugin) isMethodAllowed(method string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	for _, allowedMethod := range p.allowMethods {
		if allowedMethod == method {
			return true
		}
	}
	return false
}

func (p *CORSPlugin) areHeadersAllowed(requestedHeaders string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	headers := strings.Split(strings.ToLower(requestedHeaders), ",")
	allowedMap := make(map[string]bool)
	
	for _, header := range p.allowHeaders {
		allowedMap[strings.ToLower(strings.TrimSpace(header))] = true
	}
	
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header != "" && !allowedMap[header] {
			return false
		}
	}
	
	return true
}

func (p *CORSPlugin) setCORSHeaders(ctx *RequestContext, origin string, isPreflight bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Set origin
	if len(p.allowOrigins) == 1 && p.allowOrigins[0] == "*" && !p.allowCredentials {
		ctx.RequestCtx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	} else {
		ctx.RequestCtx.Response.Header.Set("Access-Control-Allow-Origin", origin)
		ctx.RequestCtx.Response.Header.Set("Vary", "Origin")
	}
	
	// Set credentials
	if p.allowCredentials {
		ctx.RequestCtx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	}
	
	if isPreflight {
		// Preflight headers
		ctx.RequestCtx.Response.Header.Set("Access-Control-Allow-Methods", strings.Join(p.allowMethods, ", "))
		ctx.RequestCtx.Response.Header.Set("Access-Control-Allow-Headers", strings.Join(p.allowHeaders, ", "))
		ctx.RequestCtx.Response.Header.Set("Access-Control-Max-Age", strconv.Itoa(p.maxAge))
	} else {
		// Simple request headers
		if len(p.exposeHeaders) > 0 {
			ctx.RequestCtx.Response.Header.Set("Access-Control-Expose-Headers", strings.Join(p.exposeHeaders, ", "))
		}
	}
}

func (p *CORSPlugin) corsError(ctx *RequestContext, message string) (bool, error) {
	ctx.RequestCtx.SetStatusCode(fasthttp.StatusForbidden)
	ctx.RequestCtx.SetContentType("application/json")
	
	errorResp := fmt.Sprintf(`{"error":"cors_error","message":"%s"}`, message)
	ctx.RequestCtx.SetBody([]byte(errorResp))
	
	p.logger.Warn("CORS error",
		zap.String("message", message),
		zap.String("origin", ctx.Header("Origin")),
		zap.String("method", ctx.Method()))
	
	return false, nil
}

// =============================================================================
// LOGGING PLUGIN - Structured Request/Response Logging
// =============================================================================

// LoggingPlugin provides enhanced structured logging
type LoggingPlugin struct {
	name        string
	version     string
	description string
	logger      *zap.Logger
	
	// Configuration
	logLevel         zapcore.Level
	logRequestBody   bool
	logResponseBody  bool
	maxBodySize      int64
	sensitiveHeaders map[string]bool
	sensitiveFields  map[string]bool
	logFormat        string
	includeMetrics   bool
	
	mu sync.RWMutex
}

// LoggingConfig defines configuration for the LoggingPlugin
type LoggingConfig struct {
	LogLevel         string   `json:"log_level" yaml:"log_level"`
	LogRequestBody   bool     `json:"log_request_body" yaml:"log_request_body"`
	LogResponseBody  bool     `json:"log_response_body" yaml:"log_response_body"`
	MaxBodySize      int64    `json:"max_body_size" yaml:"max_body_size"`
	SensitiveHeaders []string `json:"sensitive_headers" yaml:"sensitive_headers"`
	SensitiveFields  []string `json:"sensitive_fields" yaml:"sensitive_fields"`
	LogFormat        string   `json:"log_format" yaml:"log_format"` // "json" or "console"
	IncludeMetrics   bool     `json:"include_metrics" yaml:"include_metrics"`
}

// NewLoggingPlugin creates a new LoggingPlugin instance
func NewLoggingPlugin() Plugin {
	return &LoggingPlugin{
		name:             "logging",
		version:          BuiltinVersion,
		description:      "Structured request/response logging plugin",
		logLevel:         zapcore.InfoLevel,
		maxBodySize:      1024 * 1024, // 1MB
		sensitiveHeaders: make(map[string]bool),
		sensitiveFields:  make(map[string]bool),
		logFormat:        "json",
		includeMetrics:   true,
	}
}

func (p *LoggingPlugin) Name() string        { return p.name }
func (p *LoggingPlugin) Version() string     { return p.version }
func (p *LoggingPlugin) Description() string { return p.description }

func (p *LoggingPlugin) Init(ctx context.Context, config map[string]interface{}, logger *zap.Logger) error {
	p.logger = logger.With(zap.String("plugin", p.name))
	
	// Parse configuration
	var logConfig LoggingConfig
	if err := mapToStruct(config, &logConfig); err != nil {
		return fmt.Errorf("invalid logging config: %w", err)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Configure log level
	if logConfig.LogLevel != "" {
		if level, err := zapcore.ParseLevel(logConfig.LogLevel); err == nil {
			p.logLevel = level
		}
	}
	
	// Configure body logging
	p.logRequestBody = logConfig.LogRequestBody
	p.logResponseBody = logConfig.LogResponseBody
	
	// Configure max body size
	if logConfig.MaxBodySize > 0 {
		p.maxBodySize = logConfig.MaxBodySize
	}
	
	// Configure sensitive headers
	defaultSensitive := []string{"authorization", "cookie", "x-api-key", "x-auth-token"}
	for _, header := range defaultSensitive {
		p.sensitiveHeaders[strings.ToLower(header)] = true
	}
	for _, header := range logConfig.SensitiveHeaders {
		p.sensitiveHeaders[strings.ToLower(header)] = true
	}
	
	// Configure sensitive fields
	defaultSensitiveFields := []string{"password", "secret", "token", "key", "credential"}
	for _, field := range defaultSensitiveFields {
		p.sensitiveFields[strings.ToLower(field)] = true
	}
	for _, field := range logConfig.SensitiveFields {
		p.sensitiveFields[strings.ToLower(field)] = true
	}
	
	// Configure format
	if logConfig.LogFormat != "" {
		p.logFormat = logConfig.LogFormat
	}
	
	// Configure metrics
	p.includeMetrics = logConfig.IncludeMetrics
	
	p.logger.Info("Logging plugin initialized",
		zap.String("log_level", p.logLevel.String()),
		zap.Bool("log_request_body", p.logRequestBody),
		zap.Bool("log_response_body", p.logResponseBody),
		zap.Int64("max_body_size", p.maxBodySize),
		zap.Int("sensitive_headers", len(p.sensitiveHeaders)),
		zap.Int("sensitive_fields", len(p.sensitiveFields)))
	
	return nil
}

func (p *LoggingPlugin) Cleanup(ctx context.Context) error {
	p.logger.Info("Logging plugin cleanup completed")
	return nil
}

func (p *LoggingPlugin) Priority() Priority {
	return PriorityLow // Logging should run last
}

func (p *LoggingPlugin) PreProcess(ctx *RequestContext) (bool, error) {
	// Log request
	if p.logger.Core().Enabled(p.logLevel) {
		fields := p.buildRequestFields(ctx)
		
		switch p.logLevel {
		case zapcore.DebugLevel:
			p.logger.Debug("HTTP request", fields...)
		case zapcore.InfoLevel:
			p.logger.Info("HTTP request", fields...)
		case zapcore.WarnLevel:
			p.logger.Warn("HTTP request", fields...)
		case zapcore.ErrorLevel:
			p.logger.Error("HTTP request", fields...)
		}
	}
	
	return true, nil
}

func (p *LoggingPlugin) PostProcess(ctx *ResponseContext) error {
	// Log response
	if p.logger.Core().Enabled(p.logLevel) {
		fields := p.buildResponseFields(ctx)
		
		statusCode := ctx.RequestCtx.Response.StatusCode()
		message := "HTTP response"
		
		// Determine log level based on status code
		var logLevel zapcore.Level
		switch {
		case statusCode >= 500:
			logLevel = zapcore.ErrorLevel
		case statusCode >= 400:
			logLevel = zapcore.WarnLevel
		default:
			logLevel = p.logLevel
		}
		
		switch logLevel {
		case zapcore.DebugLevel:
			p.logger.Debug(message, fields...)
		case zapcore.InfoLevel:
			p.logger.Info(message, fields...)
		case zapcore.WarnLevel:
			p.logger.Warn(message, fields...)
		case zapcore.ErrorLevel:
			p.logger.Error(message, fields...)
		}
	}
	
	return nil
}

func (p *LoggingPlugin) ShouldApply(req *fasthttp.RequestCtx) bool {
	return true
}

func (p *LoggingPlugin) buildRequestFields(ctx *RequestContext) []zap.Field {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	fields := []zap.Field{
		zap.String("method", ctx.Method()),
		zap.String("path", ctx.Path()),
		zap.String("query", string(ctx.RequestCtx.QueryArgs().QueryString())),
		zap.String("remote_addr", ctx.RemoteAddr()),
		zap.String("user_agent", ctx.UserAgent()),
		zap.Time("timestamp", ctx.StartTime),
	}
	
	// Add request ID if available
	if requestID, exists := ctx.GetUserValue("request_id"); exists {
		fields = append(fields, zap.Any("request_id", requestID))
	}
	
	// Add user ID if available
	if userID, exists := ctx.GetUserValue("user_id"); exists {
		fields = append(fields, zap.Any("user_id", userID))
	}
	
	// Add headers (filtered)
	headers := make(map[string]string)
	ctx.RequestCtx.Request.Header.VisitAll(func(key, value []byte) {
		headerName := strings.ToLower(string(key))
		if p.sensitiveHeaders[headerName] {
			headers[headerName] = "[REDACTED]"
		} else {
			headers[headerName] = string(value)
		}
	})
	fields = append(fields, zap.Any("headers", headers))
	
	// Add request body if enabled
	if p.logRequestBody && len(ctx.Body()) > 0 {
		body := ctx.Body()
		if int64(len(body)) > p.maxBodySize {
			body = body[:p.maxBodySize]
		}
		
		// Try to parse as JSON and filter sensitive fields
		if strings.Contains(ctx.ContentType(), "application/json") {
			if filteredBody := p.filterSensitiveJSON(body); filteredBody != nil {
				fields = append(fields, zap.Any("body", json.RawMessage(filteredBody)))
			} else {
				fields = append(fields, zap.String("body", string(body)))
			}
		} else {
			fields = append(fields, zap.String("body", string(body)))
		}
	}
	
	return fields
}

func (p *LoggingPlugin) buildResponseFields(ctx *ResponseContext) []zap.Field {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	fields := []zap.Field{
		zap.String("method", ctx.Method()),
		zap.String("path", ctx.Path()),
		zap.Int("status_code", ctx.RequestCtx.Response.StatusCode()),
		zap.Duration("duration", ctx.ProcessingTime),
		zap.Int("response_size", len(ctx.RequestCtx.Response.Body())),
	}
	
	// Add request ID if available
	if requestID, exists := ctx.GetUserValue("request_id"); exists {
		fields = append(fields, zap.Any("request_id", requestID))
	}
	
	// Add user ID if available
	if userID, exists := ctx.GetUserValue("user_id"); exists {
		fields = append(fields, zap.Any("user_id", userID))
	}
	
	// Add error if occurred
	if ctx.ProcessingError != nil {
		fields = append(fields, zap.Error(ctx.ProcessingError))
	}
	
	// Add metrics if enabled
	if p.includeMetrics {
		fields = append(fields,
			zap.Int64("bytes_sent", int64(len(ctx.RequestCtx.Response.Body()))),
			zap.Int64("bytes_received", int64(len(ctx.RequestCtx.Request.Body()))))
	}
	
	// Add response headers (filtered)
	responseHeaders := make(map[string]string)
	ctx.RequestCtx.Response.Header.VisitAll(func(key, value []byte) {
		headerName := strings.ToLower(string(key))
		if p.sensitiveHeaders[headerName] {
			responseHeaders[headerName] = "[REDACTED]"
		} else {
			responseHeaders[headerName] = string(value)
		}
	})
	fields = append(fields, zap.Any("response_headers", responseHeaders))
	
	// Add response body if enabled
	if p.logResponseBody && len(ctx.ResponseBody) > 0 {
		body := ctx.ResponseBody
		if int64(len(body)) > p.maxBodySize {
			body = body[:p.maxBodySize]
		}
		
		// Try to parse as JSON and filter sensitive fields
		contentType := string(ctx.RequestCtx.Response.Header.ContentType())
		if strings.Contains(contentType, "application/json") {
			if filteredBody := p.filterSensitiveJSON(body); filteredBody != nil {
				fields = append(fields, zap.Any("response_body", json.RawMessage(filteredBody)))
			} else {
				fields = append(fields, zap.String("response_body", string(body)))
			}
		} else {
			fields = append(fields, zap.String("response_body", string(body)))
		}
	}
	
	return fields
}

func (p *LoggingPlugin) filterSensitiveJSON(data []byte) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	
	p.filterSensitiveFields(obj)
	
	filtered, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	
	return filtered
}

func (p *LoggingPlugin) filterSensitiveFields(obj map[string]interface{}) {
	for key, value := range obj {
		lowerKey := strings.ToLower(key)
		
		// Check if this field is sensitive
		isSensitive := false
		for sensitiveField := range p.sensitiveFields {
			if strings.Contains(lowerKey, sensitiveField) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			obj[key] = "[REDACTED]"
		} else if nestedObj, ok := value.(map[string]interface{}); ok {
			// Recursively filter nested objects
			p.filterSensitiveFields(nestedObj)
		} else if slice, ok := value.([]interface{}); ok {
			// Handle arrays
			for _, item := range slice {
				if nestedObj, ok := item.(map[string]interface{}); ok {
					p.filterSensitiveFields(nestedObj)
				}
			}
		}
	}
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// mapToStruct converts a map to a struct using JSON marshaling/unmarshaling
func mapToStruct(m map[string]interface{}, v interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// =============================================================================
// PLUGIN REGISTRATION FUNCTIONS
// =============================================================================

// RegisterBuiltinPlugins registers all built-in plugins with the provided registry
func RegisterBuiltinPlugins(registry *PluginRegistry) error {
	plugins := map[string]PluginFactory{
		"auth":       NewAuthPlugin,
		"rate_limit": NewRateLimitPlugin,
		"cors":       NewCORSPlugin,
		"logging":    NewLoggingPlugin,
	}
	
	for name, factory := range plugins {
		if err := registry.RegisterPlugin(name, factory); err != nil {
			return fmt.Errorf("failed to register plugin %s: %w", name, err)
		}
	}
	
	return nil
}

// GetBuiltinPluginFactories returns a map of all built-in plugin factories
func GetBuiltinPluginFactories() map[string]PluginFactory {
	return map[string]PluginFactory{
		"auth":       NewAuthPlugin,
		"rate_limit": NewRateLimitPlugin,
		"cors":       NewCORSPlugin,
		"logging":    NewLoggingPlugin,
	}
}

// CreateBuiltinPlugin creates a specific built-in plugin by name
func CreateBuiltinPlugin(name string) (Plugin, error) {
	factories := GetBuiltinPluginFactories()
	factory, exists := factories[name]
	if !exists {
		return nil, fmt.Errorf("built-in plugin not found: %s", name)
	}
	
	return factory(), nil
}

// GetBuiltinPluginNames returns a sorted list of built-in plugin names
func GetBuiltinPluginNames() []string {
	factories := GetBuiltinPluginFactories()
	names := make([]string, 0, len(factories))
	
	for name := range factories {
		names = append(names, name)
	}
	
	sort.Strings(names)
	return names
}