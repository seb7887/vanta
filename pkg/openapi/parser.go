package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SpecParser defines the interface for OpenAPI specification parsing
type SpecParser interface {
	Parse(data []byte) (*Specification, error)
	Validate(spec *Specification) error
	GetEndpoints() []Endpoint
	GetSchemas() map[string]*Schema
}

// OpenAPIParser implements SpecParser using kin-openapi
type OpenAPIParser struct {
	spec      *openapi3.T
	endpoints []Endpoint
	schemas   map[string]*Schema
}

// NewParser creates a new OpenAPI parser
func NewParser() SpecParser {
	return &OpenAPIParser{
		endpoints: make([]Endpoint, 0),
		schemas:   make(map[string]*Schema),
	}
}

// Parse parses OpenAPI specification from bytes
func (p *OpenAPIParser) Parse(data []byte) (*Specification, error) {
	// Try to determine if it's JSON or YAML
	var spec *openapi3.T
	var err error

	// First try JSON
	if json.Valid(data) {
		loader := openapi3.NewLoader()
		spec, err = loader.LoadFromData(data)
	} else {
		// Try YAML
		loader := openapi3.NewLoader()
		spec, err = loader.LoadFromData(data)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	p.spec = spec

	// Convert to our internal representation
	result, err := p.convertToInternalSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert spec: %w", err)
	}

	// Extract endpoints and schemas
	p.extractEndpoints(spec)
	p.extractSchemas(spec)

	return result, nil
}

// Validate validates the OpenAPI specification
func (p *OpenAPIParser) Validate(spec *Specification) error {
	if p.spec == nil {
		return fmt.Errorf("no specification loaded")
	}

	// Use kin-openapi's validation
	err := p.spec.Validate(nil)
	if err != nil {
		return fmt.Errorf("OpenAPI validation failed: %w", err)
	}

	return nil
}

// GetEndpoints returns all extracted endpoints
func (p *OpenAPIParser) GetEndpoints() []Endpoint {
	return p.endpoints
}

// GetSchemas returns all extracted schemas
func (p *OpenAPIParser) GetSchemas() map[string]*Schema {
	return p.schemas
}

// convertToInternalSpec converts kin-openapi spec to our internal representation
func (p *OpenAPIParser) convertToInternalSpec(spec *openapi3.T) (*Specification, error) {
	result := &Specification{
		Version: spec.OpenAPI,
		Info: InfoObject{
			Title:       spec.Info.Title,
			Description: spec.Info.Description,
			Version:     spec.Info.Version,
		},
		Paths:    make(map[string]PathItem),
		Schemas:  make(map[string]*Schema),
		Security: make([]SecurityRequirement, 0),
	}

	// Convert paths
	for path, pathItem := range spec.Paths {
		pi := PathItem{}

		if pathItem.Get != nil {
			pi.GET = p.convertOperation(pathItem.Get)
		}
		if pathItem.Post != nil {
			pi.POST = p.convertOperation(pathItem.Post)
		}
		if pathItem.Put != nil {
			pi.PUT = p.convertOperation(pathItem.Put)
		}
		if pathItem.Delete != nil {
			pi.DELETE = p.convertOperation(pathItem.Delete)
		}
		if pathItem.Patch != nil {
			pi.PATCH = p.convertOperation(pathItem.Patch)
		}

		result.Paths[path] = pi
	}

	// Convert schemas
	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, schemaRef := range spec.Components.Schemas {
			schema := p.convertSchema(schemaRef.Value)
			result.Schemas[name] = schema
		}
	}

	return result, nil
}

// convertOperation converts kin-openapi operation to our internal representation
func (p *OpenAPIParser) convertOperation(op *openapi3.Operation) *Operation {
	operation := &Operation{
		OperationID: op.OperationID,
		Summary:     op.Summary,
		Description: op.Description,
		Parameters:  make([]Parameter, 0),
		Responses:   make(map[string]Response),
	}

	// Convert parameters
	for _, paramRef := range op.Parameters {
		if paramRef.Value != nil {
			param := Parameter{
				Name:        paramRef.Value.Name,
				In:          paramRef.Value.In,
				Description: paramRef.Value.Description,
				Required:    paramRef.Value.Required,
			}
			if paramRef.Value.Schema != nil {
				param.Schema = p.convertSchema(paramRef.Value.Schema.Value)
			}
			operation.Parameters = append(operation.Parameters, param)
		}
	}

	// Convert responses
	for status, responseRef := range op.Responses {
		if responseRef.Value != nil {
			description := ""
			if responseRef.Value.Description != nil {
				description = *responseRef.Value.Description
			}
			response := Response{
				Description: description,
				Content:     make(map[string]MediaTypeObject),
				Headers:     make(map[string]Header),
			}

			// Convert content
			for mediaType, mediaTypeObj := range responseRef.Value.Content {
				mto := MediaTypeObject{}
				if mediaTypeObj.Schema != nil {
					mto.Schema = p.convertSchema(mediaTypeObj.Schema.Value)
				}
				if mediaTypeObj.Example != nil {
					mto.Example = mediaTypeObj.Example
				}
				response.Content[mediaType] = mto
			}

			operation.Responses[status] = response
		}
	}

	return operation
}

// convertSchema converts kin-openapi schema to our internal representation
func (p *OpenAPIParser) convertSchema(schema *openapi3.Schema) *Schema {
	if schema == nil {
		return nil
	}

	result := &Schema{
		Type:        schema.Type,
		Format:      schema.Format,
		Description: schema.Description,
		Pattern:     schema.Pattern,
	}

	// Convert enum
	if schema.Enum != nil {
		result.Enum = schema.Enum
	}

	// Convert default and example
	result.Default = schema.Default
	result.Example = schema.Example

	// Convert numeric constraints
	result.Minimum = schema.Min
	result.Maximum = schema.Max

	// Convert string constraints
	if schema.MinLength > 0 {
		minLen := int(schema.MinLength)
		result.MinLength = &minLen
	}
	if schema.MaxLength != nil {
		maxLen := int(*schema.MaxLength)
		result.MaxLength = &maxLen
	}

	// Convert array constraints
	if schema.MinItems > 0 {
		minItems := int(schema.MinItems)
		result.MinItems = &minItems
	}
	if schema.MaxItems != nil {
		maxItems := int(*schema.MaxItems)
		result.MaxItems = &maxItems
	}

	// Convert properties (for object types)
	if schema.Properties != nil {
		result.Properties = make(map[string]*Schema)
		for name, propRef := range schema.Properties {
			if propRef.Value != nil {
				result.Properties[name] = p.convertSchema(propRef.Value)
			}
		}
	}

	// Convert items (for array types)
	if schema.Items != nil && schema.Items.Value != nil {
		result.Items = p.convertSchema(schema.Items.Value)
	}

	// Convert required fields
	result.Required = schema.Required

	return result
}

// extractEndpoints extracts all endpoints from the specification
func (p *OpenAPIParser) extractEndpoints(spec *openapi3.T) {
	p.endpoints = make([]Endpoint, 0)

	for path, pathItem := range spec.Paths {
		methods := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, operation := range methods {
			if operation != nil {
				endpoint := Endpoint{
					Path:        path,
					Method:      strings.ToUpper(method),
					OperationID: operation.OperationID,
					Parameters:  make([]Parameter, 0),
					Responses:   make(map[string]Response),
					Operation:   p.convertOperation(operation),
				}

				// Convert parameters
				for _, paramRef := range operation.Parameters {
					if paramRef.Value != nil {
						param := Parameter{
							Name:        paramRef.Value.Name,
							In:          paramRef.Value.In,
							Description: paramRef.Value.Description,
							Required:    paramRef.Value.Required,
						}
						if paramRef.Value.Schema != nil {
							param.Schema = p.convertSchema(paramRef.Value.Schema.Value)
						}
						endpoint.Parameters = append(endpoint.Parameters, param)
					}
				}

				// Convert responses
				for status, responseRef := range operation.Responses {
					if responseRef.Value != nil {
						description := ""
						if responseRef.Value.Description != nil {
							description = *responseRef.Value.Description
						}
						response := Response{
							Description: description,
							Content:     make(map[string]MediaTypeObject),
						}

						for mediaType, mediaTypeObj := range responseRef.Value.Content {
							mto := MediaTypeObject{}
							if mediaTypeObj.Schema != nil {
								mto.Schema = p.convertSchema(mediaTypeObj.Schema.Value)
							}
							response.Content[mediaType] = mto
						}

						endpoint.Responses[status] = response
					}
				}

				p.endpoints = append(p.endpoints, endpoint)
			}
		}
	}
}

// extractSchemas extracts all schemas from the specification
func (p *OpenAPIParser) extractSchemas(spec *openapi3.T) {
	p.schemas = make(map[string]*Schema)

	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, schemaRef := range spec.Components.Schemas {
			if schemaRef.Value != nil {
				p.schemas[name] = p.convertSchema(schemaRef.Value)
			}
		}
	}
}

// LoadSpecification loads an OpenAPI specification from a file
func LoadSpecification(specPath string) (*Specification, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	parser := NewParser()
	spec, err := parser.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	return spec, nil
}

// ValidateSpecification validates an OpenAPI specification
func ValidateSpecification(spec *Specification) error {
	if spec == nil {
		return fmt.Errorf("specification cannot be nil")
	}
	
	if spec.Info.Title == "" {
		return fmt.Errorf("specification title is required")
	}
	
	if spec.Info.Version == "" {
		return fmt.Errorf("specification version is required")
	}
	
	if len(spec.Paths) == 0 {
		return fmt.Errorf("specification must have at least one path")
	}
	
	// Basic validation - could be expanded
	return nil
}