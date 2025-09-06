package openapi

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// TypeSpecificGenerators contains enhanced type-specific generation logic

// generateStringAdvanced generates a string value with advanced constraint handling
func (g *DefaultDataGenerator) generateStringAdvanced(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Check for default value first
	if schema.Default != nil {
		if str, ok := schema.Default.(string); ok {
			return str, nil
		}
	}
	
	// Determine length constraints
	minLength := 1
	maxLength := 50
	
	if schema.MinLength != nil && *schema.MinLength > 0 {
		minLength = *schema.MinLength
	}
	if schema.MaxLength != nil && *schema.MaxLength > 0 {
		maxLength = *schema.MaxLength
		if maxLength < minLength {
			maxLength = minLength
		}
	}
	
	// Handle specific patterns
	if schema.Pattern != "" {
		return g.generatePatternString(schema.Pattern, minLength, maxLength)
	}
	
	// Generate contextual string based on property name or description
	if ctx.Parent != "" {
		if contextualValue := g.generateContextualString(ctx.Parent, minLength, maxLength); contextualValue != "" {
			return contextualValue, nil
		}
	}
	
	// Generate random string within constraints
	length := g.faker.IntRange(minLength, maxLength)
	return g.faker.LetterN(uint(length)), nil
}

// generatePatternString attempts to generate a string matching the given pattern
func (g *DefaultDataGenerator) generatePatternString(pattern string, minLength, maxLength int) (string, error) {
	// For common patterns, generate appropriate strings
	switch {
	case strings.Contains(pattern, "^[a-zA-Z0-9]+$"):
		length := g.faker.IntRange(minLength, maxLength)
		return g.faker.LetterN(uint(length)), nil
	case strings.Contains(pattern, "^[0-9]+$"):
		length := g.faker.IntRange(minLength, maxLength)
		return g.faker.DigitN(uint(length)), nil
	case strings.Contains(pattern, "@"):
		// Likely email pattern
		return g.faker.Email(), nil
	case strings.Contains(pattern, "^https?://"):
		// URL pattern
		return g.faker.URL(), nil
	default:
		// For complex patterns, generate a simple string and hope it works
		// In production, consider using a proper regex-to-string generator
		length := g.faker.IntRange(minLength, maxLength)
		return g.faker.LetterN(uint(length)), nil
	}
}

// generateContextualString generates contextually appropriate strings based on field names
func (g *DefaultDataGenerator) generateContextualString(fieldName string, minLength, maxLength int) string {
	fieldLower := strings.ToLower(fieldName)
	
	switch {
	case strings.Contains(fieldLower, "email"):
		return g.faker.Email()
	case strings.Contains(fieldLower, "name"):
		if strings.Contains(fieldLower, "first") {
			return g.faker.FirstName()
		} else if strings.Contains(fieldLower, "last") {
			return g.faker.LastName()
		} else if strings.Contains(fieldLower, "company") {
			return g.faker.Company()
		}
		return g.faker.Name()
	case strings.Contains(fieldLower, "phone"):
		return g.faker.Phone()
	case strings.Contains(fieldLower, "address"):
		return g.faker.Address().Address
	case strings.Contains(fieldLower, "city"):
		return g.faker.Address().City
	case strings.Contains(fieldLower, "state"):
		return g.faker.Address().State
	case strings.Contains(fieldLower, "country"):
		return g.faker.Address().Country
	case strings.Contains(fieldLower, "zip") || strings.Contains(fieldLower, "postal"):
		return g.faker.Address().Zip
	case strings.Contains(fieldLower, "title"):
		return g.faker.JobTitle()
	case strings.Contains(fieldLower, "description"):
		return g.faker.Sentence(10)
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "link"):
		return g.faker.URL()
	case strings.Contains(fieldLower, "id") && !strings.Contains(fieldLower, "email"):
		return g.faker.UUID()
	case strings.Contains(fieldLower, "username") || strings.Contains(fieldLower, "user"):
		return g.faker.Username()
	case strings.Contains(fieldLower, "password"):
		return g.faker.Password(true, true, true, true, false, g.faker.IntRange(minLength, maxLength))
	case strings.Contains(fieldLower, "color"):
		return g.faker.Color()
	default:
		return ""
	}
}

// generateIntegerAdvanced generates an integer with advanced constraint handling
func (g *DefaultDataGenerator) generateIntegerAdvanced(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Check for default value first
	if schema.Default != nil {
		if i, ok := schema.Default.(int); ok {
			return i, nil
		}
		if f, ok := schema.Default.(float64); ok {
			return int(f), nil
		}
	}
	
	min := -1000
	max := 1000
	
	if schema.Minimum != nil {
		min = int(*schema.Minimum)
	}
	if schema.Maximum != nil {
		max = int(*schema.Maximum)
		if max < min {
			max = min
		}
	}
	
	// Generate contextual integers based on field name
	if ctx.Parent != "" {
		if contextualValue := g.generateContextualInteger(ctx.Parent, min, max); contextualValue != nil {
			return *contextualValue, nil
		}
	}
	
	return g.faker.IntRange(min, max), nil
}

// generateContextualInteger generates contextually appropriate integers
func (g *DefaultDataGenerator) generateContextualInteger(fieldName string, min, max int) *int {
	fieldLower := strings.ToLower(fieldName)
	
	switch {
	case strings.Contains(fieldLower, "age"):
		age := g.faker.IntRange(18, 80)
		if age >= min && age <= max {
			return &age
		}
	case strings.Contains(fieldLower, "year"):
		year := g.faker.IntRange(2000, 2024)
		if year >= min && year <= max {
			return &year
		}
	case strings.Contains(fieldLower, "port"):
		port := g.faker.IntRange(1024, 65535)
		if port >= min && port <= max {
			return &port
		}
	case strings.Contains(fieldLower, "count") || strings.Contains(fieldLower, "size") || strings.Contains(fieldLower, "length"):
		count := g.faker.IntRange(0, 100)
		if count >= min && count <= max {
			return &count
		}
	case strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "cost") || strings.Contains(fieldLower, "amount"):
		price := g.faker.IntRange(1, 10000)
		if price >= min && price <= max {
			return &price
		}
	case strings.Contains(fieldLower, "score") || strings.Contains(fieldLower, "rating"):
		score := g.faker.IntRange(1, 10)
		if score >= min && score <= max {
			return &score
		}
	case strings.Contains(fieldLower, "percentage") || strings.Contains(fieldLower, "percent"):
		percent := g.faker.IntRange(0, 100)
		if percent >= min && percent <= max {
			return &percent
		}
	}
	
	return nil
}

// generateNumberAdvanced generates a float64 with advanced constraint handling
func (g *DefaultDataGenerator) generateNumberAdvanced(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Check for default value first
	if schema.Default != nil {
		if f, ok := schema.Default.(float64); ok {
			return f, nil
		}
		if i, ok := schema.Default.(int); ok {
			return float64(i), nil
		}
	}
	
	min := -1000.0
	max := 1000.0
	
	if schema.Minimum != nil {
		min = *schema.Minimum
	}
	if schema.Maximum != nil {
		max = *schema.Maximum
		if max < min {
			max = min
		}
	}
	
	// Generate contextual numbers based on field name
	if ctx.Parent != "" {
		if contextualValue := g.generateContextualNumber(ctx.Parent, min, max); contextualValue != nil {
			return *contextualValue, nil
		}
	}
	
	return g.faker.Float64Range(min, max), nil
}

// generateContextualNumber generates contextually appropriate numbers
func (g *DefaultDataGenerator) generateContextualNumber(fieldName string, min, max float64) *float64 {
	fieldLower := strings.ToLower(fieldName)
	
	switch {
	case strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "cost") || strings.Contains(fieldLower, "amount"):
		price := g.faker.Float64Range(0.01, 9999.99)
		if price >= min && price <= max {
			return &price
		}
	case strings.Contains(fieldLower, "rate") || strings.Contains(fieldLower, "percentage"):
		rate := g.faker.Float64Range(0.0, 100.0)
		if rate >= min && rate <= max {
			return &rate
		}
	case strings.Contains(fieldLower, "latitude") || strings.Contains(fieldLower, "lat"):
		lat := g.faker.Float64Range(-90.0, 90.0)
		if lat >= min && lat <= max {
			return &lat
		}
	case strings.Contains(fieldLower, "longitude") || strings.Contains(fieldLower, "lon") || strings.Contains(fieldLower, "lng"):
		lng := g.faker.Float64Range(-180.0, 180.0)
		if lng >= min && lng <= max {
			return &lng
		}
	case strings.Contains(fieldLower, "weight"):
		weight := g.faker.Float64Range(0.1, 500.0)
		if weight >= min && weight <= max {
			return &weight
		}
	case strings.Contains(fieldLower, "height") || strings.Contains(fieldLower, "length"):
		height := g.faker.Float64Range(0.1, 300.0)
		if height >= min && height <= max {
			return &height
		}
	case strings.Contains(fieldLower, "temperature") || strings.Contains(fieldLower, "temp"):
		temp := g.faker.Float64Range(-50.0, 50.0)
		if temp >= min && temp <= max {
			return &temp
		}
	}
	
	return nil
}

// generateArrayAdvanced generates arrays with better element variety
func (g *DefaultDataGenerator) generateArrayAdvanced(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	if schema.Items == nil {
		return []interface{}{}, nil
	}
	
	// Determine array size
	minItems := 0
	maxItems := 3
	
	if schema.MinItems != nil && *schema.MinItems >= 0 {
		minItems = *schema.MinItems
	}
	if schema.MaxItems != nil && *schema.MaxItems >= 0 {
		maxItems = *schema.MaxItems
		if maxItems < minItems {
			maxItems = minItems
		}
	}
	
	// Use stored array size if available
	arrayKey := fmt.Sprintf("%s_%d", ctx.Parent, ctx.CurrentDepth)
	var size int
	if storedSize, exists := ctx.ArraySizes[arrayKey]; exists {
		size = storedSize
	} else {
		size = g.faker.IntRange(minItems, maxItems)
		ctx.ArraySizes[arrayKey] = size
	}
	
	result := make([]interface{}, size)
	
	// Create new context for array items
	newCtx := *ctx
	newCtx.CurrentDepth++
	
	for i := 0; i < size; i++ {
		newCtx.Parent = fmt.Sprintf("%s[%d]", ctx.Parent, i)
		item, err := g.Generate(schema.Items, &newCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate array item %d: %w", i, err)
		}
		result[i] = item
	}
	
	return result, nil
}

// generateObjectAdvanced generates objects with better property handling
func (g *DefaultDataGenerator) generateObjectAdvanced(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	result := make(map[string]interface{})
	
	if schema.Properties == nil {
		return result, nil
	}
	
	// Prevent circular references
	objectKey := fmt.Sprintf("%s_%d", ctx.Parent, ctx.CurrentDepth)
	if ctx.Visited[objectKey] {
		return nil, nil // Return nil to avoid infinite recursion
	}
	
	// Mark as visited
	ctx.Visited[objectKey] = true
	defer delete(ctx.Visited, objectKey)
	
	// Create new context for object properties
	newCtx := *ctx
	newCtx.CurrentDepth++
	
	// Create required fields lookup
	requiredFields := make(map[string]bool)
	for _, field := range schema.Required {
		requiredFields[field] = true
	}
	
	// Generate properties
	for propName, propSchema := range schema.Properties {
		// Skip if we've hit depth limit and field is not required
		if newCtx.CurrentDepth >= newCtx.MaxDepth && !requiredFields[propName] {
			continue
		}
		
		// For optional fields, decide inclusion with bias toward required-looking fields
		if !requiredFields[propName] {
			inclusionChance := 0.7 // 70% chance by default
			
			// Increase chance for commonly expected fields
			propLower := strings.ToLower(propName)
			if strings.Contains(propLower, "id") || strings.Contains(propLower, "name") ||
			   strings.Contains(propLower, "email") || strings.Contains(propLower, "created") ||
			   strings.Contains(propLower, "updated") {
				inclusionChance = 0.9
			}
			
			if g.faker.Float32Range(0, 1) > float32(inclusionChance) {
				continue
			}
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

// isAlpha checks if a string contains only alphabetic characters
func isAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// isNumeric checks if a string contains only numeric characters
func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isAlphaNumeric checks if a string contains only alphanumeric characters
func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// compileRegex safely compiles a regex pattern
func compileRegex(pattern string) (*regexp.Regexp, error) {
	// Sanitize common problematic patterns
	if strings.HasPrefix(pattern, "^") && strings.HasSuffix(pattern, "$") {
		pattern = strings.TrimPrefix(pattern, "^")
		pattern = strings.TrimSuffix(pattern, "$")
	}
	
	return regexp.Compile(pattern)
}