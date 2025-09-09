package chaos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zaptest"
	"vanta/pkg/config"
)

func TestNewDefaultChaosEngine(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	assert.NotNil(t, engine)
	assert.False(t, engine.IsEnabled())
	assert.Empty(t, engine.GetActiveScenarios())
	assert.NotNil(t, engine.injectors)
	assert.Contains(t, engine.injectors, "latency")
	assert.Contains(t, engine.injectors, "error")
}

func TestLoadScenarios(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	scenarios := []config.ScenarioConfig{
		{
			Name:        "test_latency",
			Type:        "latency",
			Endpoints:   []string{"/api/*"},
			Probability: 0.1,
			Parameters: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "100ms",
			},
		},
		{
			Name:        "test_error",
			Type:        "error",
			Endpoints:   []string{"/api/users/*"},
			Probability: 0.05,
			Parameters: map[string]interface{}{
				"error_codes": []interface{}{500, 502, 503},
			},
		},
	}
	
	err := engine.LoadScenarios(scenarios)
	require.NoError(t, err)
	
	assert.True(t, engine.IsEnabled())
	activeScenarios := engine.GetActiveScenarios()
	assert.Len(t, activeScenarios, 2)
	assert.Contains(t, activeScenarios, "test_latency")
	assert.Contains(t, activeScenarios, "test_error")
}

func TestLoadScenariosInvalidConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	tests := []struct {
		name     string
		scenario config.ScenarioConfig
		wantErr  bool
	}{
		{
			name: "empty_name",
			scenario: config.ScenarioConfig{
				Name:        "",
				Type:        "latency",
				Probability: 0.1,
			},
			wantErr: true,
		},
		{
			name: "empty_type",
			scenario: config.ScenarioConfig{
				Name:        "test",
				Type:        "",
				Probability: 0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid_probability_negative",
			scenario: config.ScenarioConfig{
				Name:        "test",
				Type:        "latency",
				Probability: -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid_probability_too_high",
			scenario: config.ScenarioConfig{
				Name:        "test",
				Type:        "latency",
				Probability: 1.5,
			},
			wantErr: true,
		},
		{
			name: "unknown_type",
			scenario: config.ScenarioConfig{
				Name:        "test",
				Type:        "unknown",
				Probability: 0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid_latency_params",
			scenario: config.ScenarioConfig{
				Name:        "test",
				Type:        "latency",
				Probability: 0.1,
				Parameters: map[string]interface{}{
					"min_delay": "invalid",
				},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.LoadScenarios([]config.ScenarioConfig{tt.scenario})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestShouldApplyChaos(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	// Test with no scenarios loaded
	should, action := engine.ShouldApplyChaos("/api/test")
	assert.False(t, should)
	assert.Empty(t, action.Type)
	
	// Load a scenario
	scenarios := []config.ScenarioConfig{
		{
			Name:        "test_latency",
			Type:        "latency",
			Endpoints:   []string{"/api/*"},
			Probability: 1.0, // 100% probability for testing
			Parameters: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "100ms",
			},
		},
	}
	
	err := engine.LoadScenarios(scenarios)
	require.NoError(t, err)
	
	// Test matching endpoint
	should, action = engine.ShouldApplyChaos("/api/test")
	assert.True(t, should)
	assert.Equal(t, "latency", action.Type)
	assert.Equal(t, "test_latency", action.Scenario)
	assert.NotEmpty(t, action.Parameters)
	
	// Test non-matching endpoint
	should, action = engine.ShouldApplyChaos("/other/test")
	assert.False(t, should)
	assert.Empty(t, action.Type)
}

func TestApplyChaos(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	// Load scenarios
	scenarios := []config.ScenarioConfig{
		{
			Name:        "test_latency",
			Type:        "latency",
			Endpoints:   []string{"/api/*"},
			Probability: 1.0,
			Parameters: map[string]interface{}{
				"min_delay": "1ms",
				"max_delay": "2ms",
			},
		},
	}
	
	err := engine.LoadScenarios(scenarios)
	require.NoError(t, err)
	
	// Create test context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/test")
	
	action := ChaosAction{
		Type:     "latency",
		Scenario: "test_latency",
		Parameters: map[string]interface{}{
			"min_delay": "1ms",
			"max_delay": "2ms",
		},
		Timestamp: time.Now(),
	}
	
	start := time.Now()
	err = engine.ApplyChaos(action, ctx)
	duration := time.Since(start)
	
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 1*time.Millisecond)
	
	// Verify statistics
	stats := engine.GetStats()
	assert.Equal(t, int64(1), stats.ChaosApplied)
	assert.Equal(t, int64(0), stats.FailedInjections)
}

func TestApplyChaosNonExistentScenario(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	ctx := &fasthttp.RequestCtx{}
	action := ChaosAction{
		Type:     "latency",
		Scenario: "non_existent",
		Parameters: map[string]interface{}{
			"min_delay": "1ms",
			"max_delay": "2ms",
		},
	}
	
	err := engine.ApplyChaos(action, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scenario not found")
}

func TestGetStats(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	stats := engine.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.ChaosApplied)
	assert.Equal(t, int64(0), stats.FailedInjections)
	assert.NotZero(t, stats.StartTime)
	assert.Empty(t, stats.ScenarioStats)
	
	// Load scenario and apply some chaos
	scenarios := []config.ScenarioConfig{
		{
			Name:        "test_scenario",
			Type:        "latency",
			Endpoints:   []string{"/api/*"},
			Probability: 1.0,
			Parameters: map[string]interface{}{
				"min_delay": "1ms",
				"max_delay": "2ms",
			},
		},
	}
	
	err := engine.LoadScenarios(scenarios)
	require.NoError(t, err)
	
	// Simulate some requests
	for i := 0; i < 5; i++ {
		_, _ = engine.ShouldApplyChaos("/api/test")
	}
	
	stats = engine.GetStats()
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.Len(t, stats.ScenarioStats, 1)
	assert.Contains(t, stats.ScenarioStats, "test_scenario")
}

func TestStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewDefaultChaosEngine(logger)
	
	// Load scenarios
	scenarios := []config.ScenarioConfig{
		{
			Name:        "test_scenario",
			Type:        "latency",
			Endpoints:   []string{"/api/*"},
			Probability: 1.0,
			Parameters: map[string]interface{}{
				"min_delay": "1ms",
				"max_delay": "2ms",
			},
		},
	}
	
	err := engine.LoadScenarios(scenarios)
	require.NoError(t, err)
	assert.True(t, engine.IsEnabled())
	assert.NotEmpty(t, engine.GetActiveScenarios())
	
	// Stop engine
	err = engine.Stop()
	assert.NoError(t, err)
	assert.False(t, engine.IsEnabled())
	assert.Empty(t, engine.GetActiveScenarios())
}

func TestEndpointMatcher(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		endpoint  string
		shouldMatch bool
	}{
		{
			name:        "exact_match",
			patterns:    []string{"/api/users"},
			endpoint:    "/api/users",
			shouldMatch: true,
		},
		{
			name:        "wildcard_match",
			patterns:    []string{"/api/*"},
			endpoint:    "/api/users",
			shouldMatch: true,
		},
		{
			name:        "deep_wildcard_match",
			patterns:    []string{"/api/*"},
			endpoint:    "/api/users/123",
			shouldMatch: true,
		},
		{
			name:        "no_match",
			patterns:    []string{"/api/*"},
			endpoint:    "/other/endpoint",
			shouldMatch: false,
		},
		{
			name:        "multiple_patterns",
			patterns:    []string{"/api/*", "/v1/*"},
			endpoint:    "/v1/users",
			shouldMatch: true,
		},
		{
			name:        "complex_pattern",
			patterns:    []string{"/api/*/users"},
			endpoint:    "/api/v1/users",
			shouldMatch: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := NewEndpointMatcher(tt.patterns)
			require.NoError(t, err)
			
			matches := matcher.Matches(tt.endpoint)
			assert.Equal(t, tt.shouldMatch, matches)
		})
	}
}

func TestEndpointMatcherInvalidPattern(t *testing.T) {
	// Test with invalid regex characters that could cause issues
	_, err := NewEndpointMatcher([]string{"[invalid"})
	assert.Error(t, err)
}