package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrKeyNotFound     = errors.New("state key not found")
	ErrInvalidValue    = errors.New("invalid value type")
	ErrStateNotEnabled = errors.New("state management not enabled")
	ErrContextExpired  = errors.New("state context expired")
)

type StateValue struct {
	Data      interface{} `json:"data"`
	TTL       time.Time   `json:"ttl,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func (sv *StateValue) IsExpired() bool {
	if sv.TTL.IsZero() {
		return false
	}
	return time.Now().After(sv.TTL)
}

type StateManager interface {
	Get(ctx context.Context, key string) (interface{}, error)
	GetWithMetadata(ctx context.Context, key string) (*StateValue, error)
	Set(ctx context.Context, key string, value interface{}) error
	SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Keys(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, key string) bool
	Size(ctx context.Context) int
	Export(ctx context.Context) (map[string]*StateValue, error)
	Import(ctx context.Context, data map[string]*StateValue) error

	// Context-scoped operations
	GetScoped(ctx context.Context, scope, key string) (interface{}, error)
	SetScoped(ctx context.Context, scope, key string, value interface{}) error
	DeleteScoped(ctx context.Context, scope, key string) error
	ClearScope(ctx context.Context, scope string) error
	ListScopes(ctx context.Context) []string

	// Lifecycle
	Start() error
	Stop() error
	IsEnabled() bool
}

type MemoryStateManager struct {
	mu           sync.RWMutex
	data         map[string]*StateValue
	scopes       map[string]map[string]*StateValue
	storage      StateStorage
	enabled      bool
	cleanupTicker *time.Ticker
	stopChan     chan struct{}
	config       *Config
}

type Config struct {
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	DefaultTTL      time.Duration `json:"default_ttl" yaml:"default_ttl"`
	Storage         StorageConfig `json:"storage" yaml:"storage"`
}

type StorageConfig struct {
	Type     string            `json:"type" yaml:"type"`
	FilePath string            `json:"file_path" yaml:"file_path"`
	Options  map[string]string `json:"options" yaml:"options"`
}

func NewMemoryStateManager(config *Config) *MemoryStateManager {
	if config == nil {
		config = &Config{
			Enabled:         true,
			CleanupInterval: 5 * time.Minute,
			DefaultTTL:      0, // No expiration by default
		}
	}

	return &MemoryStateManager{
		data:     make(map[string]*StateValue),
		scopes:   make(map[string]map[string]*StateValue),
		enabled:  config.Enabled,
		config:   config,
		stopChan: make(chan struct{}),
	}
}

func (m *MemoryStateManager) Start() error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	// Initialize storage if configured
	if m.config.Storage.Type != "" {
		storage, err := NewStorage(m.config.Storage)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		m.storage = storage

		// Load existing state
		if err := m.loadFromStorage(); err != nil {
			return fmt.Errorf("failed to load state from storage: %w", err)
		}
	}

	// Start cleanup routine
	if m.config.CleanupInterval > 0 {
		m.cleanupTicker = time.NewTicker(m.config.CleanupInterval)
		go m.cleanupRoutine()
	}

	return nil
}

func (m *MemoryStateManager) Stop() error {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	close(m.stopChan)

	// Save to storage if configured
	if m.storage != nil {
		return m.saveToStorage()
	}

	return nil
}

func (m *MemoryStateManager) IsEnabled() bool {
	return m.enabled
}

func (m *MemoryStateManager) Get(ctx context.Context, key string) (interface{}, error) {
	if !m.enabled {
		return nil, ErrStateNotEnabled
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	if value.IsExpired() {
		delete(m.data, key)
		return nil, ErrKeyNotFound
	}

	return value.Data, nil
}

func (m *MemoryStateManager) GetWithMetadata(ctx context.Context, key string) (*StateValue, error) {
	if !m.enabled {
		return nil, ErrStateNotEnabled
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	if value.IsExpired() {
		delete(m.data, key)
		return nil, ErrKeyNotFound
	}

	return value, nil
}

func (m *MemoryStateManager) Set(ctx context.Context, key string, value interface{}) error {
	return m.SetWithTTL(ctx, key, value, m.config.DefaultTTL)
}

func (m *MemoryStateManager) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	stateValue := &StateValue{
		Data:      value,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if ttl > 0 {
		stateValue.TTL = now.Add(ttl)
	}

	m.data[key] = stateValue
	return nil
}

func (m *MemoryStateManager) Delete(ctx context.Context, key string) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}

func (m *MemoryStateManager) Clear(ctx context.Context) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]*StateValue)
	return nil
}

func (m *MemoryStateManager) Keys(ctx context.Context) ([]string, error) {
	if !m.enabled {
		return nil, ErrStateNotEnabled
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.data))
	for key, value := range m.data {
		if !value.IsExpired() {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (m *MemoryStateManager) Exists(ctx context.Context, key string) bool {
	if !m.enabled {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	return exists && !value.IsExpired()
}

func (m *MemoryStateManager) Size(ctx context.Context) int {
	if !m.enabled {
		return 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, value := range m.data {
		if !value.IsExpired() {
			count++
		}
	}

	return count
}

func (m *MemoryStateManager) Export(ctx context.Context) (map[string]*StateValue, error) {
	if !m.enabled {
		return nil, ErrStateNotEnabled
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	exported := make(map[string]*StateValue)
	for key, value := range m.data {
		if !value.IsExpired() {
			exported[key] = value
		}
	}

	return exported, nil
}

func (m *MemoryStateManager) Import(ctx context.Context, data map[string]*StateValue) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, value := range data {
		if !value.IsExpired() {
			m.data[key] = value
		}
	}

	return nil
}

// Scoped operations
func (m *MemoryStateManager) GetScoped(ctx context.Context, scope, key string) (interface{}, error) {
	if !m.enabled {
		return nil, ErrStateNotEnabled
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	scopeData, exists := m.scopes[scope]
	if !exists {
		return nil, ErrKeyNotFound
	}

	value, exists := scopeData[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	if value.IsExpired() {
		delete(scopeData, key)
		return nil, ErrKeyNotFound
	}

	return value.Data, nil
}

func (m *MemoryStateManager) SetScoped(ctx context.Context, scope, key string, value interface{}) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scopes[scope] == nil {
		m.scopes[scope] = make(map[string]*StateValue)
	}

	now := time.Now()
	stateValue := &StateValue{
		Data:      value,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if m.config.DefaultTTL > 0 {
		stateValue.TTL = now.Add(m.config.DefaultTTL)
	}

	m.scopes[scope][key] = stateValue
	return nil
}

func (m *MemoryStateManager) DeleteScoped(ctx context.Context, scope, key string) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if scopeData, exists := m.scopes[scope]; exists {
		delete(scopeData, key)
	}

	return nil
}

func (m *MemoryStateManager) ClearScope(ctx context.Context, scope string) error {
	if !m.enabled {
		return ErrStateNotEnabled
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.scopes, scope)
	return nil
}

func (m *MemoryStateManager) ListScopes(ctx context.Context) []string {
	if !m.enabled {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	scopes := make([]string, 0, len(m.scopes))
	for scope := range m.scopes {
		scopes = append(scopes, scope)
	}

	return scopes
}

// Private methods
func (m *MemoryStateManager) cleanupRoutine() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.cleanupExpired()
		case <-m.stopChan:
			return
		}
	}
}

func (m *MemoryStateManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up global state
	for key, value := range m.data {
		if value.IsExpired() {
			delete(m.data, key)
		}
	}

	// Clean up scoped state
	for scope, scopeData := range m.scopes {
		for key, value := range scopeData {
			if value.IsExpired() {
				delete(scopeData, key)
			}
		}
		// Remove empty scopes
		if len(scopeData) == 0 {
			delete(m.scopes, scope)
		}
	}
}

func (m *MemoryStateManager) loadFromStorage() error {
	if m.storage == nil {
		return nil
	}

	data, err := m.storage.Load()
	if err != nil {
		return err
	}

	if len(data) > 0 {
		return m.Import(context.Background(), data)
	}

	return nil
}

func (m *MemoryStateManager) saveToStorage() error {
	if m.storage == nil {
		return nil
	}

	data, err := m.Export(context.Background())
	if err != nil {
		return err
	}

	return m.storage.Save(data)
}
