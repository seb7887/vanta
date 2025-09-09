package chaos

import (
	"time"
	"vanta/pkg/config"
	"github.com/valyala/fasthttp"
)

// ChaosEngine is the main interface for chaos testing functionality
type ChaosEngine interface {
	// LoadScenarios loads chaos scenarios from configuration
	LoadScenarios(scenarios []config.ScenarioConfig) error
	
	// ShouldApplyChaos determines if chaos should be applied to the given endpoint
	ShouldApplyChaos(endpoint string) (bool, ChaosAction)
	
	// ApplyChaos applies the specified chaos action to the request context
	ApplyChaos(action ChaosAction, ctx *fasthttp.RequestCtx) error
	
	// GetActiveScenarios returns a list of currently active scenario names
	GetActiveScenarios() []string
	
	// Stop stops the chaos engine and cleans up resources
	Stop() error
	
	// IsEnabled returns whether the chaos engine is currently enabled
	IsEnabled() bool
	
	// GetStats returns chaos engine statistics
	GetStats() EngineStats
}

// Injector defines the interface for chaos injectors
type Injector interface {
	// Inject applies chaos to the request context using the provided parameters
	Inject(ctx *fasthttp.RequestCtx, params map[string]interface{}) error
	
	// Type returns the type of chaos this injector handles
	Type() string
	
	// Validate validates the parameters for this injector
	Validate(params map[string]interface{}) error
}

// ChaosAction represents an action to be applied by the chaos engine
type ChaosAction struct {
	Type       string                 `json:"type"`
	Scenario   string                 `json:"scenario"`
	Parameters map[string]interface{} `json:"parameters"`
	Timestamp  time.Time              `json:"timestamp"`
}

// EngineStats contains statistics about chaos engine operations
type EngineStats struct {
	TotalRequests    int64                    `json:"total_requests"`
	ChaosApplied     int64                    `json:"chaos_applied"`
	FailedInjections int64                    `json:"failed_injections"`
	ScenarioStats    map[string]ScenarioStats `json:"scenario_stats"`
	StartTime        time.Time                `json:"start_time"`
}

// ScenarioStats contains statistics for a specific scenario
type ScenarioStats struct {
	Name         string    `json:"name"`
	AppliedCount int64     `json:"applied_count"`
	FailedCount  int64     `json:"failed_count"`
	LastApplied  time.Time `json:"last_applied"`
}

// InjectionResult represents the result of a chaos injection
type InjectionResult struct {
	Success   bool          `json:"success"`
	Error     error         `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Scenario  string        `json:"scenario"`
	Type      string        `json:"type"`
	Timestamp time.Time     `json:"timestamp"`
}