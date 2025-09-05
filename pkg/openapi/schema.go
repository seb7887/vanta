package openapi

import "time"

// Specification represents a parsed OpenAPI specification
type Specification struct {
	Version   string                `json:"version"`
	Info      InfoObject            `json:"info"`
	Paths     map[string]PathItem   `json:"paths"`
	Schemas   map[string]*Schema    `json:"schemas"`
	Security  []SecurityRequirement `json:"security"`
}

// InfoObject represents the info section of OpenAPI spec
type InfoObject struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// PathItem represents a single path in OpenAPI spec
type PathItem struct {
	GET    *Operation `json:"get,omitempty"`
	POST   *Operation `json:"post,omitempty"`
	PUT    *Operation `json:"put,omitempty"`
	DELETE *Operation `json:"delete,omitempty"`
	PATCH  *Operation `json:"patch,omitempty"`
}

// Operation represents an operation on a path
type Operation struct {
	OperationID string              `json:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter represents a parameter in an operation
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // query, header, path, cookie
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody represents a request body
type RequestBody struct {
	Description string                       `json:"description,omitempty"`
	Content     map[string]MediaTypeObject   `json:"content"`
	Required    bool                         `json:"required,omitempty"`
}

// Response represents a response
type Response struct {
	Description string                       `json:"description"`
	Content     map[string]MediaTypeObject   `json:"content,omitempty"`
	Headers     map[string]Header            `json:"headers,omitempty"`
}

// MediaTypeObject represents a media type
type MediaTypeObject struct {
	Schema  *Schema     `json:"schema,omitempty"`
	Example interface{} `json:"example,omitempty"`
}

// Header represents a header
type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// Schema represents a JSON Schema
type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Description          string            `json:"description,omitempty"`
	Enum                 []interface{}     `json:"enum,omitempty"`
	Default              interface{}       `json:"default,omitempty"`
	Example              interface{}       `json:"example,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Required             []string          `json:"required,omitempty"`
	AdditionalProperties interface{}       `json:"additionalProperties,omitempty"`
	Pattern              string            `json:"pattern,omitempty"`
	Minimum              *float64          `json:"minimum,omitempty"`
	Maximum              *float64          `json:"maximum,omitempty"`
	MinItems             *int              `json:"minItems,omitempty"`
	MaxItems             *int              `json:"maxItems,omitempty"`
	MinLength            *int              `json:"minLength,omitempty"`
	MaxLength            *int              `json:"maxLength,omitempty"`
}

// SecurityRequirement represents a security requirement
type SecurityRequirement map[string][]string

// Endpoint represents a single API endpoint
type Endpoint struct {
	Path        string
	Method      string
	OperationID string
	Parameters  []Parameter
	Responses   map[string]Response
	Operation   *Operation
}

// GenerationContext provides context for data generation
type GenerationContext struct {
	MaxDepth     int
	CurrentDepth int
	Visited      map[string]bool
	Locale       string
	Seed         int64
	Timestamp    time.Time
}