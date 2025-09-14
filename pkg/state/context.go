package state

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ContextKey string

const (
	StateContextKey     ContextKey = "state_context"
	SessionContextKey   ContextKey = "session_context"
	EndpointContextKey  ContextKey = "endpoint_context"
	RequestContextKey   ContextKey = "request_context"
)

type StateContext struct {
	SessionID    string            `json:"session_id"`
	EndpointPath string            `json:"endpoint_path"`
	RequestID    string            `json:"request_id"`
	UserID       string            `json:"user_id,omitempty"`
	Metadata     map[string]string `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at,omitempty"`
}

func (sc *StateContext) IsExpired() bool {
	if sc.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(sc.ExpiresAt)
}

func (sc *StateContext) GetScope() string {
	parts := []string{}

	if sc.SessionID != "" {
		parts = append(parts, "session:"+sc.SessionID)
	}

	if sc.EndpointPath != "" {
		parts = append(parts, "endpoint:"+sc.EndpointPath)
	}

	if sc.UserID != "" {
		parts = append(parts, "user:"+sc.UserID)
	}

	return strings.Join(parts, "/")
}

func (sc *StateContext) GetSessionScope() string {
	if sc.SessionID == "" {
		return ""
	}
	return "session:" + sc.SessionID
}

func (sc *StateContext) GetEndpointScope() string {
	if sc.EndpointPath == "" {
		return ""
	}
	return "endpoint:" + sc.EndpointPath
}

func (sc *StateContext) GetUserScope() string {
	if sc.UserID == "" {
		return ""
	}
	return "user:" + sc.UserID
}

type ContextManager struct {
	contexts map[string]*StateContext
	mu       sync.RWMutex
	config   *ContextConfig
}

type ContextConfig struct {
	DefaultTTL      time.Duration `json:"default_ttl" yaml:"default_ttl"`
	SessionTTL      time.Duration `json:"session_ttl" yaml:"session_ttl"`
	RequestTTL      time.Duration `json:"request_ttl" yaml:"request_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
}

func NewContextManager(config *ContextConfig) *ContextManager {
	if config == nil {
		config = &ContextConfig{
			DefaultTTL:      30 * time.Minute,
			SessionTTL:      24 * time.Hour,
			RequestTTL:      5 * time.Minute,
			CleanupInterval: 5 * time.Minute,
		}
	}

	return &ContextManager{
		contexts: make(map[string]*StateContext),
		config:   config,
	}
}

func (cm *ContextManager) CreateContext(sessionID, endpointPath, requestID, userID string) *StateContext {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	context := &StateContext{
		SessionID:    sessionID,
		EndpointPath: endpointPath,
		RequestID:    requestID,
		UserID:       userID,
		Metadata:     make(map[string]string),
		CreatedAt:    now,
		ExpiresAt:    now.Add(cm.config.DefaultTTL),
	}

	key := cm.generateContextKey(sessionID, endpointPath, requestID)
	cm.contexts[key] = context

	return context
}

func (cm *ContextManager) GetContext(sessionID, endpointPath, requestID string) (*StateContext, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	key := cm.generateContextKey(sessionID, endpointPath, requestID)
	context, exists := cm.contexts[key]

	if !exists || context.IsExpired() {
		return nil, false
	}

	return context, true
}

func (cm *ContextManager) UpdateContext(sessionID, endpointPath, requestID string, updates map[string]string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := cm.generateContextKey(sessionID, endpointPath, requestID)
	context, exists := cm.contexts[key]

	if !exists || context.IsExpired() {
		return fmt.Errorf("context not found or expired")
	}

	for k, v := range updates {
		context.Metadata[k] = v
	}

	return nil
}

func (cm *ContextManager) DeleteContext(sessionID, endpointPath, requestID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := cm.generateContextKey(sessionID, endpointPath, requestID)
	delete(cm.contexts, key)
}

func (cm *ContextManager) ListContexts() []*StateContext {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	contexts := make([]*StateContext, 0, len(cm.contexts))
	for _, context := range cm.contexts {
		if !context.IsExpired() {
			contexts = append(contexts, context)
		}
	}

	return contexts
}

func (cm *ContextManager) CleanupExpired() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cleaned := 0
	for key, context := range cm.contexts {
		if context.IsExpired() {
			delete(cm.contexts, key)
			cleaned++
		}
	}

	return cleaned
}

func (cm *ContextManager) generateContextKey(sessionID, endpointPath, requestID string) string {
	return fmt.Sprintf("%s:%s:%s", sessionID, endpointPath, requestID)
}

// Context helpers for HTTP middleware
func WithStateContext(ctx context.Context, stateCtx *StateContext) context.Context {
	return context.WithValue(ctx, StateContextKey, stateCtx)
}

func GetStateContext(ctx context.Context) (*StateContext, bool) {
	stateCtx, ok := ctx.Value(StateContextKey).(*StateContext)
	return stateCtx, ok
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionContextKey, sessionID)
}

func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(SessionContextKey).(string)
	return sessionID, ok
}

func WithEndpointPath(ctx context.Context, endpointPath string) context.Context {
	return context.WithValue(ctx, EndpointContextKey, endpointPath)
}

func GetEndpointPath(ctx context.Context) (string, bool) {
	endpointPath, ok := ctx.Value(EndpointContextKey).(string)
	return endpointPath, ok
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestContextKey, requestID)
}

func GetRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(RequestContextKey).(string)
	return requestID, ok
}

// Scoped state operations using context
type ScopedStateManager struct {
	stateManager   StateManager
	contextManager *ContextManager
}

func NewScopedStateManager(stateManager StateManager, contextManager *ContextManager) *ScopedStateManager {
	return &ScopedStateManager{
		stateManager:   stateManager,
		contextManager: contextManager,
	}
}

func (ssm *ScopedStateManager) GetFromContext(ctx context.Context, key string) (interface{}, error) {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return ssm.stateManager.Get(ctx, key)
	}

	scope := stateCtx.GetScope()
	if scope == "" {
		return ssm.stateManager.Get(ctx, key)
	}

	// Try scoped first, then fall back to global
	value, err := ssm.stateManager.GetScoped(ctx, scope, key)
	if err == ErrKeyNotFound {
		return ssm.stateManager.Get(ctx, key)
	}

	return value, err
}

func (ssm *ScopedStateManager) SetInContext(ctx context.Context, key string, value interface{}) error {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return ssm.stateManager.Set(ctx, key, value)
	}

	scope := stateCtx.GetScope()
	if scope == "" {
		return ssm.stateManager.Set(ctx, key, value)
	}

	return ssm.stateManager.SetScoped(ctx, scope, key, value)
}

func (ssm *ScopedStateManager) GetFromSession(ctx context.Context, key string) (interface{}, error) {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no state context found")
	}

	sessionScope := stateCtx.GetSessionScope()
	if sessionScope == "" {
		return nil, fmt.Errorf("no session scope available")
	}

	return ssm.stateManager.GetScoped(ctx, sessionScope, key)
}

func (ssm *ScopedStateManager) SetInSession(ctx context.Context, key string, value interface{}) error {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return fmt.Errorf("no state context found")
	}

	sessionScope := stateCtx.GetSessionScope()
	if sessionScope == "" {
		return fmt.Errorf("no session scope available")
	}

	return ssm.stateManager.SetScoped(ctx, sessionScope, key, value)
}

func (ssm *ScopedStateManager) GetFromEndpoint(ctx context.Context, key string) (interface{}, error) {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no state context found")
	}

	endpointScope := stateCtx.GetEndpointScope()
	if endpointScope == "" {
		return nil, fmt.Errorf("no endpoint scope available")
	}

	return ssm.stateManager.GetScoped(ctx, endpointScope, key)
}

func (ssm *ScopedStateManager) SetInEndpoint(ctx context.Context, key string, value interface{}) error {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return fmt.Errorf("no state context found")
	}

	endpointScope := stateCtx.GetEndpointScope()
	if endpointScope == "" {
		return fmt.Errorf("no endpoint scope available")
	}

	return ssm.stateManager.SetScoped(ctx, endpointScope, key, value)
}

func (ssm *ScopedStateManager) GetFromUser(ctx context.Context, key string) (interface{}, error) {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no state context found")
	}

	userScope := stateCtx.GetUserScope()
	if userScope == "" {
		return nil, fmt.Errorf("no user scope available")
	}

	return ssm.stateManager.GetScoped(ctx, userScope, key)
}

func (ssm *ScopedStateManager) SetInUser(ctx context.Context, key string, value interface{}) error {
	stateCtx, ok := GetStateContext(ctx)
	if !ok {
		return fmt.Errorf("no state context found")
	}

	userScope := stateCtx.GetUserScope()
	if userScope == "" {
		return fmt.Errorf("no user scope available")
	}

	return ssm.stateManager.SetScoped(ctx, userScope, key, value)
}