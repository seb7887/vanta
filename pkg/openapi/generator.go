package openapi

import (
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

// DataGenerator defines the interface for mock data generation
type DataGenerator interface {
	// Generate creates mock data based on the provided schema and context
	Generate(schema *Schema, ctx *GenerationContext) (interface{}, error)
	
	// SetSeed sets the random seed for deterministic generation
	SetSeed(seed int64)
	
	// SetLocale sets the locale for data generation
	SetLocale(locale string)
}

// FormatGenerator defines a function type for format-specific data generation
type FormatGenerator func(schema *Schema, ctx *GenerationContext) (interface{}, error)

// DefaultDataGenerator is the default implementation of DataGenerator
type DefaultDataGenerator struct {
	faker            *gofakeit.Faker
	formatGenerators map[string]FormatGenerator
	locale           string
	seed             int64
}

// NewDefaultDataGenerator creates a new DefaultDataGenerator instance
func NewDefaultDataGenerator() *DefaultDataGenerator {
	seed := time.Now().UnixNano()
	faker := gofakeit.New(seed)
	
	generator := &DefaultDataGenerator{
		faker:            faker,
		formatGenerators: make(map[string]FormatGenerator),
		locale:           "en",
		seed:             seed,
	}
	
	// Register default format generators
	generator.registerDefaultFormats()
	
	return generator
}

// NewDefaultDataGeneratorWithSeed creates a new DefaultDataGenerator with a specific seed
func NewDefaultDataGeneratorWithSeed(seed int64) *DefaultDataGenerator {
	faker := gofakeit.New(seed)
	
	generator := &DefaultDataGenerator{
		faker:            faker,
		formatGenerators: make(map[string]FormatGenerator),
		locale:           "en",
		seed:             seed,
	}
	
	// Register default format generators
	generator.registerDefaultFormats()
	
	return generator
}

// Generate generates mock data based on the provided schema and context
func (g *DefaultDataGenerator) Generate(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}
	
	if ctx == nil {
		ctx = &GenerationContext{
			MaxDepth:     5,
			CurrentDepth: 0,
			Visited:      make(map[string]bool),
			ArraySizes:   make(map[string]int),
			Locale:       g.locale,
			Seed:         g.seed,
			Timestamp:    time.Now(),
		}
	}
	
	// Check for circular references
	if ctx.CurrentDepth >= ctx.MaxDepth {
		return nil, nil
	}
	
	// Prioritize example if available
	if schema.Example != nil {
		return schema.Example, nil
	}
	
	// Handle enum values
	if len(schema.Enum) > 0 {
		return schema.Enum[g.faker.IntRange(0, len(schema.Enum)-1)], nil
	}
	
	// Check for format-specific generators first
	if schema.Format != "" {
		if formatGen, exists := g.formatGenerators[schema.Format]; exists {
			return formatGen(schema, ctx)
		}
	}
	
	// Generate based on type
	switch schema.Type {
	case "string":
		return g.generateString(schema, ctx)
	case "integer":
		return g.generateInteger(schema, ctx)
	case "number":
		return g.generateNumber(schema, ctx)
	case "boolean":
		return g.generateBoolean(schema, ctx)
	case "array":
		return g.generateArray(schema, ctx)
	case "object":
		return g.generateObject(schema, ctx)
	case "":
		// No type specified, try to infer or generate a generic value
		if schema.Properties != nil {
			return g.generateObject(schema, ctx)
		}
		if schema.Items != nil {
			return g.generateArray(schema, ctx)
		}
		// Default to string
		return g.generateString(schema, ctx)
	default:
		return nil, fmt.Errorf("unsupported schema type: %s", schema.Type)
	}
}

// SetSeed sets the random seed for deterministic generation
func (g *DefaultDataGenerator) SetSeed(seed int64) {
	g.seed = seed
	g.faker = gofakeit.New(seed)
}

// SetLocale sets the locale for data generation
func (g *DefaultDataGenerator) SetLocale(locale string) {
	g.locale = locale
	// Note: gofakeit doesn't directly support locale changes after initialization
	// This is stored for future use in custom generators
}

// GetSeed returns the current seed value
func (g *DefaultDataGenerator) GetSeed() int64 {
	return g.seed
}

// RegisterFormatGenerator registers a custom format generator
func (g *DefaultDataGenerator) RegisterFormatGenerator(format string, generator FormatGenerator) {
	g.formatGenerators[format] = generator
}

// generateString generates a string value based on schema constraints
func (g *DefaultDataGenerator) generateString(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	var result string
	
	// Determine length constraints
	minLength := 1
	maxLength := 50
	
	if schema.MinLength != nil && *schema.MinLength > 0 {
		minLength = *schema.MinLength
	}
	if schema.MaxLength != nil && *schema.MaxLength > 0 {
		maxLength = *schema.MaxLength
		// Ensure maxLength is not less than minLength
		if maxLength < minLength {
			maxLength = minLength
		}
	}
	
	// Handle pattern constraint
	if schema.Pattern != "" {
		// For patterns, we'll generate a simple string and hope it matches
		// In a production system, you might want to use a regex-based generator
		result = g.faker.LetterN(uint(g.faker.IntRange(minLength, maxLength)))
	} else {
		// Generate a random string within length constraints
		length := g.faker.IntRange(minLength, maxLength)
		result = g.faker.LetterN(uint(length))
	}
	
	return result, nil
}

// generateInteger generates an integer value based on schema constraints
func (g *DefaultDataGenerator) generateInteger(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	min := -1000
	max := 1000
	
	if schema.Minimum != nil {
		min = int(*schema.Minimum)
	}
	if schema.Maximum != nil {
		max = int(*schema.Maximum)
		// Ensure max is not less than min
		if max < min {
			max = min
		}
	}
	
	return g.faker.IntRange(min, max), nil
}

// generateNumber generates a float64 value based on schema constraints
func (g *DefaultDataGenerator) generateNumber(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	min := -1000.0
	max := 1000.0
	
	if schema.Minimum != nil {
		min = *schema.Minimum
	}
	if schema.Maximum != nil {
		max = *schema.Maximum
		// Ensure max is not less than min
		if max < min {
			max = min
		}
	}
	
	return g.faker.Float64Range(min, max), nil
}

// generateBoolean generates a boolean value
func (g *DefaultDataGenerator) generateBoolean(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.Bool(), nil
}

// generateArray generates an array value based on schema constraints
func (g *DefaultDataGenerator) generateArray(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	if schema.Items == nil {
		return []interface{}{}, nil
	}
	
	// Determine array size constraints
	minItems := 1
	maxItems := 3
	defaultArraySize := 2
	
	if schema.MinItems != nil && *schema.MinItems >= 0 {
		minItems = *schema.MinItems
	}
	if schema.MaxItems != nil && *schema.MaxItems >= 0 {
		maxItems = *schema.MaxItems
		// Ensure maxItems is not less than minItems
		if maxItems < minItems {
			maxItems = minItems
		}
	}
	
	// Use configured default array size if within constraints
	if defaultArraySize >= minItems && defaultArraySize <= maxItems {
		maxItems = defaultArraySize
		minItems = defaultArraySize
	}
	
	size := g.faker.IntRange(minItems, maxItems)
	result := make([]interface{}, size)
	
	// Create new context for array items
	newCtx := *ctx
	newCtx.CurrentDepth++
	
	for i := 0; i < size; i++ {
		item, err := g.Generate(schema.Items, &newCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate array item %d: %w", i, err)
		}
		result[i] = item
	}
	
	return result, nil
}

// generateObject generates an object value based on schema properties
func (g *DefaultDataGenerator) generateObject(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	result := make(map[string]interface{})
	
	if schema.Properties == nil {
		return result, nil
	}
	
	// Create new context for object properties
	newCtx := *ctx
	newCtx.CurrentDepth++
	
	// Create a map for required fields lookup
	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}
	
	// Generate properties
	for propName, propSchema := range schema.Properties {
		// Skip if we've hit depth limit and this is not a required field
		if newCtx.CurrentDepth >= newCtx.MaxDepth && !requiredFields[propName] {
			continue
		}
		
		// For optional fields, randomly decide whether to include them (70% chance)
		if !requiredFields[propName] && g.faker.Float32Range(0, 1) > 0.7 {
			continue
		}
		
		// Set context for property generation
		newCtx.Required = requiredFields[propName]
		newCtx.Parent = propName
		
		value, err := g.Generate(propSchema, &newCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate property '%s': %w", propName, err)
		}
		
		if value != nil {
			result[propName] = value
		}
	}
	
	return result, nil
}

