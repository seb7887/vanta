package openapi

import (
	"fmt"
	"strings"
	"time"
)

// Standard OpenAPI formats as defined in the specification
const (
	// String formats
	FormatDateTime = "date-time"
	FormatDate     = "date"
	FormatTime     = "time"
	FormatEmail    = "email"
	FormatURI      = "uri"
	FormatURL      = "url"
	FormatUUID     = "uuid"
	FormatHostname = "hostname"
	FormatIPv4     = "ipv4"
	FormatIPv6     = "ipv6"
	FormatPassword = "password"
	FormatByte     = "byte"
	FormatBinary   = "binary"
	
	// Number formats
	FormatFloat  = "float"
	FormatDouble = "double"
	
	// Integer formats
	FormatInt32 = "int32"
	FormatInt64 = "int64"
)

// registerDefaultFormats registers all standard OpenAPI format generators
func (g *DefaultDataGenerator) registerDefaultFormats() {
	// String formats
	g.RegisterFormatGenerator(FormatDateTime, g.generateDateTime)
	g.RegisterFormatGenerator(FormatDate, g.generateDate)
	g.RegisterFormatGenerator(FormatTime, g.generateTime)
	g.RegisterFormatGenerator(FormatEmail, g.generateEmail)
	g.RegisterFormatGenerator(FormatURI, g.generateURI)
	g.RegisterFormatGenerator(FormatURL, g.generateURL)
	g.RegisterFormatGenerator(FormatUUID, g.generateUUID)
	g.RegisterFormatGenerator(FormatHostname, g.generateHostname)
	g.RegisterFormatGenerator(FormatIPv4, g.generateIPv4)
	g.RegisterFormatGenerator(FormatIPv6, g.generateIPv6)
	g.RegisterFormatGenerator(FormatPassword, g.generatePassword)
	g.RegisterFormatGenerator(FormatByte, g.generateByte)
	g.RegisterFormatGenerator(FormatBinary, g.generateBinary)
	
	// Number formats
	g.RegisterFormatGenerator(FormatFloat, g.generateFloat)
	g.RegisterFormatGenerator(FormatDouble, g.generateDouble)
	
	// Integer formats  
	g.RegisterFormatGenerator(FormatInt32, g.generateInt32)
	g.RegisterFormatGenerator(FormatInt64, g.generateInt64)
	
	// Additional common formats (not in OpenAPI spec but commonly used)
	g.RegisterFormatGenerator("username", g.generateUsername)
	g.RegisterFormatGenerator("slug", g.generateSlug)
	g.RegisterFormatGenerator("color", g.generateColor)
	g.RegisterFormatGenerator("phone", g.generatePhone)
	g.RegisterFormatGenerator("credit-card", g.generateCreditCard)
	g.RegisterFormatGenerator("iban", g.generateIBAN)
}

// Date/Time format generators

func (g *DefaultDataGenerator) generateDateTime(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate RFC3339 formatted datetime
	now := time.Now()
	randomTime := g.faker.DateRange(now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0))
	return randomTime.Format(time.RFC3339), nil
}

func (g *DefaultDataGenerator) generateDate(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate RFC3339 date (YYYY-MM-DD)
	now := time.Now()
	randomTime := g.faker.DateRange(now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0))
	return randomTime.Format("2006-01-02"), nil
}

func (g *DefaultDataGenerator) generateTime(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate RFC3339 time (HH:MM:SS)
	hour := g.faker.IntRange(0, 23)
	minute := g.faker.IntRange(0, 59)
	second := g.faker.IntRange(0, 59)
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second), nil
}

// Network format generators

func (g *DefaultDataGenerator) generateEmail(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.Email(), nil
}

func (g *DefaultDataGenerator) generateURI(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.URL(), nil
}

func (g *DefaultDataGenerator) generateURL(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.URL(), nil
}

func (g *DefaultDataGenerator) generateUUID(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.UUID(), nil
}

func (g *DefaultDataGenerator) generateHostname(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.DomainName(), nil
}

func (g *DefaultDataGenerator) generateIPv4(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.IPv4Address(), nil
}

func (g *DefaultDataGenerator) generateIPv6(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.IPv6Address(), nil
}

// Security format generators

func (g *DefaultDataGenerator) generatePassword(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	minLength := 8
	maxLength := 32
	
	if schema.MinLength != nil && *schema.MinLength > 0 {
		minLength = *schema.MinLength
	}
	if schema.MaxLength != nil && *schema.MaxLength > 0 {
		maxLength = *schema.MaxLength
		if maxLength < minLength {
			maxLength = minLength
		}
	}
	
	length := g.faker.IntRange(minLength, maxLength)
	return g.faker.Password(true, true, true, true, false, length), nil
}

func (g *DefaultDataGenerator) generateByte(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate base64 encoded string
	length := g.faker.IntRange(10, 50)
	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = byte(g.faker.IntRange(0, 255))
	}
	return fmt.Sprintf("%x", bytes), nil
}

func (g *DefaultDataGenerator) generateBinary(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate binary data representation
	length := g.faker.IntRange(10, 100)
	return g.faker.LetterN(uint(length)), nil
}

// Number format generators

func (g *DefaultDataGenerator) generateFloat(schema *Schema, ctx *GenerationContext) (interface{}, error) {
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
	
	return float32(g.faker.Float64Range(min, max)), nil
}

func (g *DefaultDataGenerator) generateDouble(schema *Schema, ctx *GenerationContext) (interface{}, error) {
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
	
	return g.faker.Float64Range(min, max), nil
}

// Integer format generators

func (g *DefaultDataGenerator) generateInt32(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	min := -2147483648
	max := 2147483647
	
	if schema.Minimum != nil {
		if *schema.Minimum > float64(min) {
			min = int(*schema.Minimum)
		}
	}
	if schema.Maximum != nil {
		if *schema.Maximum < float64(max) {
			max = int(*schema.Maximum)
		}
		if max < min {
			max = min
		}
	}
	
	return int32(g.faker.IntRange(min, max)), nil
}

func (g *DefaultDataGenerator) generateInt64(schema *Schema, ctx *GenerationContext) (interface{}, error) {
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
	
	return int64(g.faker.IntRange(min, max)), nil
}

// Additional common format generators

func (g *DefaultDataGenerator) generateUsername(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.Username(), nil
}

func (g *DefaultDataGenerator) generateSlug(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	words := g.faker.IntRange(2, 4)
	parts := make([]string, words)
	for i := 0; i < words; i++ {
		parts[i] = strings.ToLower(g.faker.Word())
	}
	return strings.Join(parts, "-"), nil
}

func (g *DefaultDataGenerator) generateColor(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate hex color
	return g.faker.HexColor(), nil
}

func (g *DefaultDataGenerator) generatePhone(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.Phone(), nil
}

func (g *DefaultDataGenerator) generateCreditCard(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	return g.faker.CreditCardNumber(nil), nil
}

func (g *DefaultDataGenerator) generateIBAN(schema *Schema, ctx *GenerationContext) (interface{}, error) {
	// Generate a simple IBAN-like string (not a real IBAN)
	country := g.faker.CountryAbr()
	checkDigits := g.faker.DigitN(2)
	bankCode := g.faker.DigitN(4)
	accountNumber := g.faker.DigitN(10)
	
	return fmt.Sprintf("%s%s%s%s", country, checkDigits, bankCode, accountNumber), nil
}

// Custom format registration helpers

// RegisterCustomFormat allows registration of custom format generators
func (g *DefaultDataGenerator) RegisterCustomFormat(format string, generator FormatGenerator) {
	g.RegisterFormatGenerator(format, generator)
}

// GetRegisteredFormats returns a list of all registered formats
func (g *DefaultDataGenerator) GetRegisteredFormats() []string {
	formats := make([]string, 0, len(g.formatGenerators))
	for format := range g.formatGenerators {
		formats = append(formats, format)
	}
	return formats
}

// HasFormatGenerator checks if a format generator is registered
func (g *DefaultDataGenerator) HasFormatGenerator(format string) bool {
	_, exists := g.formatGenerators[format]
	return exists
}

// RemoveFormatGenerator removes a format generator
func (g *DefaultDataGenerator) RemoveFormatGenerator(format string) {
	delete(g.formatGenerators, format)
}

// FormatGeneratorInfo provides information about format generators
type FormatGeneratorInfo struct {
	Format      string `json:"format"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// GetFormatGeneratorInfo returns information about all registered format generators
func (g *DefaultDataGenerator) GetFormatGeneratorInfo() []FormatGeneratorInfo {
	infos := []FormatGeneratorInfo{
		{FormatDateTime, "RFC3339 date-time format", "2023-12-25T10:30:00Z"},
		{FormatDate, "RFC3339 date format", "2023-12-25"},
		{FormatTime, "RFC3339 time format", "10:30:00"},
		{FormatEmail, "Email address", "user@example.com"},
		{FormatURI, "URI/URL", "https://example.com/path"},
		{FormatURL, "URL", "https://example.com"},
		{FormatUUID, "UUID v4", "123e4567-e89b-12d3-a456-426614174000"},
		{FormatHostname, "Domain name", "example.com"},
		{FormatIPv4, "IPv4 address", "192.168.1.1"},
		{FormatIPv6, "IPv6 address", "2001:db8::1"},
		{FormatPassword, "Password string", "P@ssw0rd123"},
		{FormatByte, "Base64 encoded bytes", "YWJjZGVmZ2g="},
		{FormatBinary, "Binary data", "binary content"},
		{FormatFloat, "32-bit floating point", "123.45"},
		{FormatDouble, "64-bit floating point", "123.456789"},
		{FormatInt32, "32-bit integer", "2147483647"},
		{FormatInt64, "64-bit integer", "9223372036854775807"},
		{"username", "Username", "johndoe"},
		{"slug", "URL slug", "my-article-title"},
		{"color", "Hex color", "#FF5733"},
		{"phone", "Phone number", "+1-555-123-4567"},
		{"credit-card", "Credit card number", "4111111111111111"},
		{"iban", "IBAN-like string", "DE89123456781234567890"},
	}
	
	// Only return info for registered formats
	result := make([]FormatGeneratorInfo, 0)
	for _, info := range infos {
		if g.HasFormatGenerator(info.Format) {
			result = append(result, info)
		}
	}
	
	return result
}