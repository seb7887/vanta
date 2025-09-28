package openapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParserLoadsMediaTypeExamples(t *testing.T) {
	specPath := filepath.Join("..", "..", "examples", "petstore.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read spec: %v", err)
	}

	parser := NewParser()
	spec, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	pathItem, exists := spec.Paths["/pets"]
	if !exists {
		t.Fatalf("expected /pets path in spec")
	}

	operation := pathItem.GET
	if operation == nil {
		t.Fatalf("expected GET operation for /pets")
	}

	response, exists := operation.Responses["200"]
	if !exists {
		t.Fatalf("expected 200 response for /pets")
	}

	media, exists := response.Content["application/json"]
	if !exists {
		t.Fatalf("expected application/json content for /pets 200")
	}

	if len(media.Examples) == 0 {
		t.Fatalf("expected examples to be parsed from media type")
	}

	if _, ok := media.Examples["small_list"]; !ok {
		t.Fatalf("expected example 'small_list' to be present")
	}
}
