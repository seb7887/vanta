package chaos

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap/zaptest"
)

func TestNewLatencyInjector(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	
	injector := NewLatencyInjector(logger, rng)
	assert.NotNil(t, injector)
	assert.Equal(t, "latency", injector.Type())
}

func TestLatencyInjectorValidate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid_params",
			params: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "100ms",
			},
			wantErr: false,
		},
		{
			name: "missing_min_delay",
			params: map[string]interface{}{
				"max_delay": "100ms",
			},
			wantErr: true,
		},
		{
			name: "missing_max_delay",
			params: map[string]interface{}{
				"min_delay": "10ms",
			},
			wantErr: true,
		},
		{
			name: "invalid_min_delay_type",
			params: map[string]interface{}{
				"min_delay": 123,
				"max_delay": "100ms",
			},
			wantErr: true,
		},
		{
			name: "invalid_max_delay_type",
			params: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": 123,
			},
			wantErr: true,
		},
		{
			name: "invalid_min_delay_format",
			params: map[string]interface{}{
				"min_delay": "invalid",
				"max_delay": "100ms",
			},
			wantErr: true,
		},
		{
			name: "invalid_max_delay_format",
			params: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "invalid",
			},
			wantErr: true,
		},
		{
			name: "negative_min_delay",
			params: map[string]interface{}{
				"min_delay": "-10ms",
				"max_delay": "100ms",
			},
			wantErr: true,
		},
		{
			name: "negative_max_delay",
			params: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "-100ms",
			},
			wantErr: true,
		},
		{
			name: "min_greater_than_max",
			params: map[string]interface{}{
				"min_delay": "200ms",
				"max_delay": "100ms",
			},
			wantErr: true,
		},
		{
			name: "equal_delays",
			params: map[string]interface{}{
				"min_delay": "100ms",
				"max_delay": "100ms",
			},
			wantErr: false,
		},
		{
			name: "zero_delays",
			params: map[string]interface{}{
				"min_delay": "0ms",
				"max_delay": "0ms",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLatencyInjectorInject(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/test/path")
	
	params := map[string]interface{}{
		"min_delay": "10ms",
		"max_delay": "50ms",
	}
	
	start := time.Now()
	err := injector.Inject(ctx, params)
	duration := time.Since(start)
	
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
	assert.LessOrEqual(t, duration, 100*time.Millisecond) // Allow some margin for test execution
}

func TestLatencyInjectorInjectInvalidParams(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	
	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "invalid_min_delay",
			params: map[string]interface{}{
				"min_delay": "invalid",
				"max_delay": "50ms",
			},
		},
		{
			name: "invalid_max_delay",
			params: map[string]interface{}{
				"min_delay": "10ms",
				"max_delay": "invalid",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := injector.Inject(ctx, tt.params)
			assert.Error(t, err)
		})
	}
}

func TestCalculateRandomDelay(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	tests := []struct {
		name     string
		min      time.Duration
		max      time.Duration
		numTests int
	}{
		{
			name:     "equal_delays",
			min:      50 * time.Millisecond,
			max:      50 * time.Millisecond,
			numTests: 10,
		},
		{
			name:     "different_delays",
			min:      10 * time.Millisecond,
			max:      100 * time.Millisecond,
			numTests: 100,
		},
		{
			name:     "zero_min",
			min:      0,
			max:      50 * time.Millisecond,
			numTests: 50,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.numTests; i++ {
				delay := injector.calculateRandomDelay(tt.min, tt.max)
				assert.GreaterOrEqual(t, delay, tt.min)
				assert.LessOrEqual(t, delay, tt.max)
			}
		})
	}
}

func TestLatencyInjectorWithZeroDelay(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	params := map[string]interface{}{
		"min_delay": "0ms",
		"max_delay": "0ms",
	}
	
	start := time.Now()
	err := injector.Inject(ctx, params)
	duration := time.Since(start)
	
	assert.NoError(t, err)
	// Should be very fast with zero delay
	assert.LessOrEqual(t, duration, 10*time.Millisecond)
}

func TestLatencyInjectorTypeMethod(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	assert.Equal(t, "latency", injector.Type())
}

func BenchmarkLatencyInjectorInject(b *testing.B) {
	logger := zaptest.NewLogger(b)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	ctx := &fasthttp.RequestCtx{}
	params := map[string]interface{}{
		"min_delay": "1ms",
		"max_delay": "2ms",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = injector.Inject(ctx, params)
	}
}

func BenchmarkCalculateRandomDelay(b *testing.B) {
	logger := zaptest.NewLogger(b)
	rng := rand.New(rand.NewSource(1))
	injector := NewLatencyInjector(logger, rng)
	
	min := 10 * time.Millisecond
	max := 100 * time.Millisecond
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = injector.calculateRandomDelay(min, max)
	}
}