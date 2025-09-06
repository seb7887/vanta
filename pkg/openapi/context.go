package openapi

import (
	"fmt"
	"time"
)

// GenerationContext provides enhanced context for data generation with improved circular reference detection
type GenerationContextEnhanced struct {
	MaxDepth         int               `json:"max_depth"`
	CurrentDepth     int               `json:"current_depth"`
	Visited          map[string]bool   `json:"visited"`
	ArraySizes       map[string]int    `json:"array_sizes"`
	Required         bool              `json:"required"`
	Parent           string            `json:"parent"`
	Locale           string            `json:"locale"`
	Seed             int64             `json:"seed"`
	Timestamp        time.Time         `json:"timestamp"`
	PreferExamples   bool              `json:"prefer_examples"`
	DefaultArraySize int               `json:"default_array_size"`
	CircularRefCount map[string]int    `json:"circular_ref_count"`
	Path             []string          `json:"path"`
}

// NewGenerationContext creates a new generation context with default values
func NewGenerationContext() *GenerationContext {
	return &GenerationContext{
		MaxDepth:     5,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
		Locale:       "en",
		Seed:         time.Now().UnixNano(),
		Timestamp:    time.Now(),
	}
}

// NewGenerationContextWithConfig creates a new generation context with configuration
func NewGenerationContextWithConfig(maxDepth int, seed int64, locale string, defaultArraySize int, preferExamples bool) *GenerationContext {
	return &GenerationContext{
		MaxDepth:     maxDepth,
		CurrentDepth: 0,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
		Locale:       locale,
		Seed:         seed,
		Timestamp:    time.Now(),
	}
}

// Clone creates a deep copy of the generation context
func (ctx *GenerationContext) Clone() *GenerationContext {
	newCtx := &GenerationContext{
		MaxDepth:     ctx.MaxDepth,
		CurrentDepth: ctx.CurrentDepth,
		Required:     ctx.Required,
		Parent:       ctx.Parent,
		Locale:       ctx.Locale,
		Seed:         ctx.Seed,
		Timestamp:    ctx.Timestamp,
		Visited:      make(map[string]bool),
		ArraySizes:   make(map[string]int),
	}
	
	// Deep copy maps
	for k, v := range ctx.Visited {
		newCtx.Visited[k] = v
	}
	for k, v := range ctx.ArraySizes {
		newCtx.ArraySizes[k] = v
	}
	
	return newCtx
}

// WithDepthIncrement returns a new context with incremented depth
func (ctx *GenerationContext) WithDepthIncrement() *GenerationContext {
	newCtx := ctx.Clone()
	newCtx.CurrentDepth++
	return newCtx
}

// WithParent returns a new context with a new parent path
func (ctx *GenerationContext) WithParent(parent string) *GenerationContext {
	newCtx := ctx.Clone()
	newCtx.Parent = parent
	return newCtx
}

// WithRequired returns a new context with required flag set
func (ctx *GenerationContext) WithRequired(required bool) *GenerationContext {
	newCtx := ctx.Clone()
	newCtx.Required = required
	return newCtx
}

// IsCircularReference checks if we're in a circular reference situation
func (ctx *GenerationContext) IsCircularReference(key string) bool {
	return ctx.Visited[key]
}

// MarkVisited marks a key as visited to prevent circular references
func (ctx *GenerationContext) MarkVisited(key string) {
	ctx.Visited[key] = true
}

// UnmarkVisited removes a key from visited to allow revisiting in different paths
func (ctx *GenerationContext) UnmarkVisited(key string) {
	delete(ctx.Visited, key)
}

// GetArraySize returns the stored array size for a given key, or 0 if not set
func (ctx *GenerationContext) GetArraySize(key string) int {
	return ctx.ArraySizes[key]
}

// SetArraySize stores an array size for a given key
func (ctx *GenerationContext) SetArraySize(key string, size int) {
	ctx.ArraySizes[key] = size
}

// HasArraySize checks if an array size is stored for a given key
func (ctx *GenerationContext) HasArraySize(key string) bool {
	_, exists := ctx.ArraySizes[key]
	return exists
}

// ShouldSkipOptionalField determines if an optional field should be skipped based on context
func (ctx *GenerationContext) ShouldSkipOptionalField(fieldName string) bool {
	if ctx.Required {
		return false
	}
	
	// At maximum depth, skip optional fields more aggressively
	if ctx.CurrentDepth >= ctx.MaxDepth-1 {
		return true
	}
	
	return false
}

// GetContextKey generates a unique key for the current context state
func (ctx *GenerationContext) GetContextKey(schemaType string) string {
	return fmt.Sprintf("%s_%s_%d", ctx.Parent, schemaType, ctx.CurrentDepth)
}

// GetObjectKey generates a unique key for object tracking
func (ctx *GenerationContext) GetObjectKey(objectName string) string {
	return fmt.Sprintf("obj_%s_%s_%d", objectName, ctx.Parent, ctx.CurrentDepth)
}

// GetArrayKey generates a unique key for array tracking
func (ctx *GenerationContext) GetArrayKey(arrayName string) string {
	return fmt.Sprintf("arr_%s_%s_%d", arrayName, ctx.Parent, ctx.CurrentDepth)
}

// IsAtMaxDepth checks if the current depth has reached the maximum
func (ctx *GenerationContext) IsAtMaxDepth() bool {
	return ctx.CurrentDepth >= ctx.MaxDepth
}

// IsNearMaxDepth checks if the current depth is close to maximum (within 1 level)
func (ctx *GenerationContext) IsNearMaxDepth() bool {
	return ctx.CurrentDepth >= ctx.MaxDepth-1
}

// Reset resets the context to initial state but preserves configuration
func (ctx *GenerationContext) Reset() {
	ctx.CurrentDepth = 0
	ctx.Parent = ""
	ctx.Required = false
	ctx.Visited = make(map[string]bool)
	ctx.ArraySizes = make(map[string]int)
	ctx.Timestamp = time.Now()
}

// GetDepthRemaining returns the number of depth levels remaining
func (ctx *GenerationContext) GetDepthRemaining() int {
	remaining := ctx.MaxDepth - ctx.CurrentDepth
	if remaining < 0 {
		return 0
	}
	return remaining
}

// String returns a string representation of the context for debugging
func (ctx *GenerationContext) String() string {
	return fmt.Sprintf("GenerationContext{Depth: %d/%d, Parent: %s, Required: %t, Visited: %d, ArraySizes: %d}",
		ctx.CurrentDepth, ctx.MaxDepth, ctx.Parent, ctx.Required, len(ctx.Visited), len(ctx.ArraySizes))
}

// ValidationResult represents the result of context validation
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// Validate validates the generation context configuration
func (ctx *GenerationContext) Validate() ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: make([]string, 0),
	}
	
	if ctx.MaxDepth <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "MaxDepth must be greater than 0")
	}
	
	if ctx.MaxDepth > 20 {
		result.Valid = false
		result.Errors = append(result.Errors, "MaxDepth should not exceed 20 to prevent excessive generation")
	}
	
	if ctx.CurrentDepth < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "CurrentDepth cannot be negative")
	}
	
	if ctx.Visited == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "Visited map cannot be nil")
	}
	
	if ctx.ArraySizes == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "ArraySizes map cannot be nil")
	}
	
	if ctx.Locale == "" {
		result.Errors = append(result.Errors, "Locale is empty, defaulting to 'en'")
	}
	
	return result
}

// ContextStats provides statistics about the generation context
type ContextStats struct {
	MaxDepth         int `json:"max_depth"`
	CurrentDepth     int `json:"current_depth"`
	DepthRemaining   int `json:"depth_remaining"`
	VisitedCount     int `json:"visited_count"`
	ArraySizesCount  int `json:"array_sizes_count"`
	IsAtMaxDepth     bool `json:"is_at_max_depth"`
	IsNearMaxDepth   bool `json:"is_near_max_depth"`
}

// GetStats returns statistics about the current context state
func (ctx *GenerationContext) GetStats() ContextStats {
	return ContextStats{
		MaxDepth:         ctx.MaxDepth,
		CurrentDepth:     ctx.CurrentDepth,
		DepthRemaining:   ctx.GetDepthRemaining(),
		VisitedCount:     len(ctx.Visited),
		ArraySizesCount:  len(ctx.ArraySizes),
		IsAtMaxDepth:     ctx.IsAtMaxDepth(),
		IsNearMaxDepth:   ctx.IsNearMaxDepth(),
	}
}