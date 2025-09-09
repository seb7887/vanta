package chaos

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// LatencyInjector implements chaos injection by adding artificial latency to requests
type LatencyInjector struct {
	logger *zap.Logger
	rng    *rand.Rand
}

// NewLatencyInjector creates a new latency injector
func NewLatencyInjector(logger *zap.Logger, rng *rand.Rand) *LatencyInjector {
	return &LatencyInjector{
		logger: logger,
		rng:    rng,
	}
}

// Type returns the type of chaos this injector handles
func (l *LatencyInjector) Type() string {
	return "latency"
}

// Validate validates the parameters for latency injection
func (l *LatencyInjector) Validate(params map[string]interface{}) error {
	minDelayRaw, hasMin := params["min_delay"]
	maxDelayRaw, hasMax := params["max_delay"]
	
	if !hasMin || !hasMax {
		return fmt.Errorf("latency injector requires 'min_delay' and 'max_delay' parameters")
	}
	
	minDelayStr, ok := minDelayRaw.(string)
	if !ok {
		return fmt.Errorf("min_delay must be a string duration (e.g., '10ms')")
	}
	
	maxDelayStr, ok := maxDelayRaw.(string)
	if !ok {
		return fmt.Errorf("max_delay must be a string duration (e.g., '100ms')")
	}
	
	minDelay, err := time.ParseDuration(minDelayStr)
	if err != nil {
		return fmt.Errorf("invalid min_delay format: %w", err)
	}
	
	maxDelay, err := time.ParseDuration(maxDelayStr)
	if err != nil {
		return fmt.Errorf("invalid max_delay format: %w", err)
	}
	
	if minDelay < 0 {
		return fmt.Errorf("min_delay cannot be negative")
	}
	
	if maxDelay < 0 {
		return fmt.Errorf("max_delay cannot be negative")
	}
	
	if minDelay > maxDelay {
		return fmt.Errorf("min_delay (%v) cannot be greater than max_delay (%v)", minDelay, maxDelay)
	}
	
	return nil
}

// Inject applies latency chaos to the request context
func (l *LatencyInjector) Inject(ctx *fasthttp.RequestCtx, params map[string]interface{}) error {
	minDelayStr := params["min_delay"].(string)
	maxDelayStr := params["max_delay"].(string)
	
	minDelay, err := time.ParseDuration(minDelayStr)
	if err != nil {
		return fmt.Errorf("failed to parse min_delay: %w", err)
	}
	
	maxDelay, err := time.ParseDuration(maxDelayStr)
	if err != nil {
		return fmt.Errorf("failed to parse max_delay: %w", err)
	}
	
	// Calculate random delay between min and max
	delay := l.calculateRandomDelay(minDelay, maxDelay)
	
	l.logger.Debug("Injecting latency chaos",
		zap.String("path", string(ctx.Path())),
		zap.Duration("delay", delay),
		zap.Duration("min_delay", minDelay),
		zap.Duration("max_delay", maxDelay))
	
	// Add delay
	time.Sleep(delay)
	
	return nil
}

// calculateRandomDelay calculates a random delay between min and max durations
func (l *LatencyInjector) calculateRandomDelay(min, max time.Duration) time.Duration {
	if min == max {
		return min
	}
	
	// Convert to nanoseconds for calculation
	minNs := min.Nanoseconds()
	maxNs := max.Nanoseconds()
	
	// Generate random value between 0 and (max-min)
	randomNs := l.rng.Int63n(maxNs - minNs)
	
	// Add to minimum to get final delay
	return time.Duration(minNs + randomNs)
}