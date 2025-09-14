package state

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMemoryStateManager_BasicOperations(t *testing.T) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Second,
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()

	// Test Set/Get
	key := "test_key"
	value := "test_value"

	err := manager.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	retrievedValue, err := manager.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("Expected %v, got %v", value, retrievedValue)
	}

	// Test Exists
	if !manager.Exists(ctx, key) {
		t.Error("Key should exist")
	}

	// Test Delete
	err = manager.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	if manager.Exists(ctx, key) {
		t.Error("Key should not exist after deletion")
	}

	_, err = manager.Get(ctx, key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestMemoryStateManager_TTL(t *testing.T) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 100 * time.Millisecond,
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	key := "ttl_key"
	value := "ttl_value"
	ttl := 200 * time.Millisecond

	// Set value with TTL
	err := manager.SetWithTTL(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Failed to set value with TTL: %v", err)
	}

	// Value should exist immediately
	if !manager.Exists(ctx, key) {
		t.Error("Key should exist immediately after setting")
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 100*time.Millisecond)

	// Value should be expired
	if manager.Exists(ctx, key) {
		t.Error("Key should not exist after TTL expiration")
	}

	_, err = manager.Get(ctx, key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestMemoryStateManager_ScopedOperations(t *testing.T) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Second,
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	scope := "test_scope"
	key := "scoped_key"
	value := "scoped_value"

	// Test scoped set/get
	err := manager.SetScoped(ctx, scope, key, value)
	if err != nil {
		t.Fatalf("Failed to set scoped value: %v", err)
	}

	retrievedValue, err := manager.GetScoped(ctx, scope, key)
	if err != nil {
		t.Fatalf("Failed to get scoped value: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("Expected %v, got %v", value, retrievedValue)
	}

	// Test scope isolation - same key in different scope should not exist
	_, err = manager.GetScoped(ctx, "other_scope", key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound for different scope, got %v", err)
	}

	// Test list scopes
	scopes := manager.ListScopes(ctx)
	found := false
	for _, s := range scopes {
		if s == scope {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Scope %s not found in list: %v", scope, scopes)
	}

	// Test clear scope
	err = manager.ClearScope(ctx, scope)
	if err != nil {
		t.Fatalf("Failed to clear scope: %v", err)
	}

	_, err = manager.GetScoped(ctx, scope, key)
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound after scope clear, got %v", err)
	}
}

func TestMemoryStateManager_ExportImport(t *testing.T) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Second,
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()

	// Set some test data
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": map[string]interface{}{"nested": true},
	}

	for key, value := range testData {
		err := manager.Set(ctx, key, value)
		if err != nil {
			t.Fatalf("Failed to set test data: %v", err)
		}
	}

	// Export state
	exported, err := manager.Export(ctx)
	if err != nil {
		t.Fatalf("Failed to export state: %v", err)
	}

	if len(exported) != len(testData) {
		t.Errorf("Expected %d exported items, got %d", len(testData), len(exported))
	}

	// Clear state and import
	err = manager.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear state: %v", err)
	}

	if manager.Size(ctx) != 0 {
		t.Error("State should be empty after clear")
	}

	err = manager.Import(ctx, exported)
	if err != nil {
		t.Fatalf("Failed to import state: %v", err)
	}

	// Verify imported data
	if manager.Size(ctx) != len(testData) {
		t.Errorf("Expected %d items after import, got %d", len(testData), manager.Size(ctx))
	}

	for key, expectedValue := range testData {
		actualValue, err := manager.Get(ctx, key)
		if err != nil {
			t.Errorf("Failed to get imported key %s: %v", key, err)
			continue
		}

		// Note: JSON unmarshaling may change number types
		if key == "key2" {
			if int(actualValue.(float64)) != expectedValue {
				t.Errorf("Key %s: expected %v, got %v", key, expectedValue, actualValue)
			}
		} else {
			if actualValue != expectedValue {
				t.Errorf("Key %s: expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	}
}

func TestMemoryStateManager_Concurrency(t *testing.T) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Second,
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	numGoroutines := 10
	numOperations := 100

	// Test concurrent reads and writes
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)

				// Set
				err := manager.Set(ctx, key, value)
				if err != nil {
					t.Errorf("Goroutine %d: failed to set %s: %v", id, key, err)
				}

				// Get
				retrievedValue, err := manager.Get(ctx, key)
				if err != nil {
					t.Errorf("Goroutine %d: failed to get %s: %v", id, key, err)
				} else if retrievedValue != value {
					t.Errorf("Goroutine %d: expected %v, got %v", id, value, retrievedValue)
				}

				// Delete
				err = manager.Delete(ctx, key)
				if err != nil {
					t.Errorf("Goroutine %d: failed to delete %s: %v", id, key, err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// State should be empty
	if manager.Size(ctx) != 0 {
		t.Errorf("Expected empty state, got size %d", manager.Size(ctx))
	}
}

func BenchmarkMemoryStateManager_Set(b *testing.B) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Hour, // Disable cleanup for benchmark
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		b.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	value := "benchmark_value"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("benchmark_key_%d", i)
		err := manager.Set(ctx, key, value)
		if err != nil {
			b.Fatalf("Failed to set value: %v", err)
		}
	}
}

func BenchmarkMemoryStateManager_Get(b *testing.B) {
	config := &Config{
		Enabled:         true,
		CleanupInterval: 1 * time.Hour, // Disable cleanup for benchmark
		DefaultTTL:      0,
	}

	manager := NewMemoryStateManager(config)
	if err := manager.Start(); err != nil {
		b.Fatalf("Failed to start state manager: %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	key := "benchmark_key"
	value := "benchmark_value"

	// Set up test data
	err := manager.Set(ctx, key, value)
	if err != nil {
		b.Fatalf("Failed to set test data: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.Get(ctx, key)
		if err != nil {
			b.Fatalf("Failed to get value: %v", err)
		}
	}
}