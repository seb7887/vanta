package chaos

import (
	"fmt"
	"math/rand"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"vanta/pkg/config"
)

// DefaultChaosEngine is the default implementation of ChaosEngine
type DefaultChaosEngine struct {
	scenarios map[string]*ChaosScenario
	injectors map[string]Injector
	enabled   bool
	
	// Thread-safe counters
	totalRequests    int64
	chaosApplied     int64
	failedInjections int64
	
	// Concurrency control
	mu sync.RWMutex
	
	// Dependencies
	logger    *zap.Logger
	rng       *rand.Rand
	startTime time.Time
}

// ChaosScenario represents an active chaos scenario
type ChaosScenario struct {
	Config       config.ScenarioConfig
	Injector     Injector
	Matcher      *EndpointMatcher
	LastApplied  time.Time
	ApplyCount   int64
	FailedCount  int64
}

// EndpointMatcher handles endpoint pattern matching
type EndpointMatcher struct {
	patterns []string
	compiled []*regexp.Regexp
}

// NewDefaultChaosEngine creates a new default chaos engine
func NewDefaultChaosEngine(logger *zap.Logger) *DefaultChaosEngine {
	engine := &DefaultChaosEngine{
		scenarios: make(map[string]*ChaosScenario),
		injectors: make(map[string]Injector),
		enabled:   false,
		logger:    logger,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		startTime: time.Now(),
	}
	
	// Register built-in injectors
	engine.registerBuiltinInjectors()
	
	return engine
}

// registerBuiltinInjectors registers the built-in chaos injectors
func (e *DefaultChaosEngine) registerBuiltinInjectors() {
	// These will be implemented in separate files
	e.injectors["latency"] = NewLatencyInjector(e.logger, e.rng)
	e.injectors["error"] = NewErrorInjector(e.logger, e.rng)
}

// LoadScenarios loads chaos scenarios from configuration
func (e *DefaultChaosEngine) LoadScenarios(scenarios []config.ScenarioConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.logger.Info("Loading chaos scenarios", zap.Int("count", len(scenarios)))
	
	// Clear existing scenarios
	e.scenarios = make(map[string]*ChaosScenario)
	
	for _, scenarioConfig := range scenarios {
		if err := e.loadScenario(scenarioConfig); err != nil {
			e.logger.Error("Failed to load scenario", 
				zap.String("name", scenarioConfig.Name),
				zap.Error(err))
			continue
		}
	}
	
	e.enabled = len(e.scenarios) > 0
	e.logger.Info("Chaos scenarios loaded successfully", 
		zap.Int("active_scenarios", len(e.scenarios)),
		zap.Bool("enabled", e.enabled))
	
	return nil
}

// loadScenario loads a single scenario
func (e *DefaultChaosEngine) loadScenario(scenarioConfig config.ScenarioConfig) error {
	// Validate scenario configuration
	if scenarioConfig.Name == "" {
		return fmt.Errorf("scenario name cannot be empty")
	}
	
	if scenarioConfig.Type == "" {
		return fmt.Errorf("scenario type cannot be empty")
	}
	
	if scenarioConfig.Probability < 0 || scenarioConfig.Probability > 1 {
		return fmt.Errorf("scenario probability must be between 0 and 1")
	}
	
	// Get injector for this scenario type
	injector, exists := e.injectors[scenarioConfig.Type]
	if !exists {
		return fmt.Errorf("unknown scenario type: %s", scenarioConfig.Type)
	}
	
	// Validate injector parameters
	if err := injector.Validate(scenarioConfig.Parameters); err != nil {
		return fmt.Errorf("invalid parameters for %s injector: %w", scenarioConfig.Type, err)
	}
	
	// Create endpoint matcher
	matcher, err := NewEndpointMatcher(scenarioConfig.Endpoints)
	if err != nil {
		return fmt.Errorf("failed to create endpoint matcher: %w", err)
	}
	
	// Create and store scenario
	scenario := &ChaosScenario{
		Config:   scenarioConfig,
		Injector: injector,
		Matcher:  matcher,
	}
	
	e.scenarios[scenarioConfig.Name] = scenario
	
	e.logger.Debug("Loaded chaos scenario",
		zap.String("name", scenarioConfig.Name),
		zap.String("type", scenarioConfig.Type),
		zap.Float64("probability", scenarioConfig.Probability),
		zap.Strings("endpoints", scenarioConfig.Endpoints))
	
	return nil
}

// ShouldApplyChaos determines if chaos should be applied to the given endpoint
func (e *DefaultChaosEngine) ShouldApplyChaos(endpoint string) (bool, ChaosAction) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	atomic.AddInt64(&e.totalRequests, 1)
	
	if !e.enabled || len(e.scenarios) == 0 {
		return false, ChaosAction{}
	}
	
	// Check each scenario
	for _, scenario := range e.scenarios {
		if scenario.Matcher.Matches(endpoint) {
			// Check probability
			if e.rng.Float64() <= scenario.Config.Probability {
				action := ChaosAction{
					Type:       scenario.Config.Type,
					Scenario:   scenario.Config.Name,
					Parameters: scenario.Config.Parameters,
					Timestamp:  time.Now(),
				}
				return true, action
			}
		}
	}
	
	return false, ChaosAction{}
}

// ApplyChaos applies the specified chaos action to the request context
func (e *DefaultChaosEngine) ApplyChaos(action ChaosAction, ctx *fasthttp.RequestCtx) error {
	e.mu.RLock()
	scenario, exists := e.scenarios[action.Scenario]
	e.mu.RUnlock()
	
	if !exists {
		atomic.AddInt64(&e.failedInjections, 1)
		return fmt.Errorf("scenario not found: %s", action.Scenario)
	}
	
	start := time.Now()
	err := scenario.Injector.Inject(ctx, action.Parameters)
	duration := time.Since(start)
	
	if err != nil {
		atomic.AddInt64(&scenario.FailedCount, 1)
		atomic.AddInt64(&e.failedInjections, 1)
		e.logger.Error("Chaos injection failed",
			zap.String("scenario", action.Scenario),
			zap.String("type", action.Type),
			zap.Duration("duration", duration),
			zap.Error(err))
		return err
	}
	
	// Update statistics
	atomic.AddInt64(&scenario.ApplyCount, 1)
	atomic.AddInt64(&e.chaosApplied, 1)
	scenario.LastApplied = time.Now()
	
	e.logger.Debug("Chaos applied successfully",
		zap.String("scenario", action.Scenario),
		zap.String("type", action.Type),
		zap.Duration("duration", duration))
	
	return nil
}

// GetActiveScenarios returns a list of currently active scenario names
func (e *DefaultChaosEngine) GetActiveScenarios() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	scenarios := make([]string, 0, len(e.scenarios))
	for name := range e.scenarios {
		scenarios = append(scenarios, name)
	}
	return scenarios
}

// IsEnabled returns whether the chaos engine is currently enabled
func (e *DefaultChaosEngine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// GetStats returns chaos engine statistics
func (e *DefaultChaosEngine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	scenarioStats := make(map[string]ScenarioStats)
	for name, scenario := range e.scenarios {
		scenarioStats[name] = ScenarioStats{
			Name:         name,
			AppliedCount: atomic.LoadInt64(&scenario.ApplyCount),
			FailedCount:  atomic.LoadInt64(&scenario.FailedCount),
			LastApplied:  scenario.LastApplied,
		}
	}
	
	return EngineStats{
		TotalRequests:    atomic.LoadInt64(&e.totalRequests),
		ChaosApplied:     atomic.LoadInt64(&e.chaosApplied),
		FailedInjections: atomic.LoadInt64(&e.failedInjections),
		ScenarioStats:    scenarioStats,
		StartTime:        e.startTime,
	}
}

// Stop stops the chaos engine and cleans up resources
func (e *DefaultChaosEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.enabled = false
	e.scenarios = make(map[string]*ChaosScenario)
	
	e.logger.Info("Chaos engine stopped")
	return nil
}

// NewEndpointMatcher creates a new endpoint matcher
func NewEndpointMatcher(patterns []string) (*EndpointMatcher, error) {
	matcher := &EndpointMatcher{
		patterns: patterns,
		compiled: make([]*regexp.Regexp, 0, len(patterns)),
	}
	
	for _, pattern := range patterns {
		// Convert glob-like patterns to regex
		// Replace * with .* and escape other regex characters
		regexPattern := regexp.QuoteMeta(pattern)
		regexPattern = "^" + regexPattern + "$"
		regexPattern = regexp.MustCompile(`\\\*`).ReplaceAllString(regexPattern, ".*")
		
		compiled, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		
		matcher.compiled = append(matcher.compiled, compiled)
	}
	
	return matcher, nil
}

// Matches checks if the given endpoint matches any of the patterns
func (m *EndpointMatcher) Matches(endpoint string) bool {
	for _, regex := range m.compiled {
		if regex.MatchString(endpoint) {
			return true
		}
	}
	return false
}