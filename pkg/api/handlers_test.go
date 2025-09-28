package api

import (
	"testing"

	"github.com/seb7887/vanta/pkg/openapi"
)

func TestResolveExampleSelection(t *testing.T) {
	schemaWithExamples := &openapi.Schema{
		Examples: map[string]openapi.ExampleObject{
			"beta":  {Value: map[string]any{"id": 2}},
			"alpha": {Value: map[string]any{"id": 1}},
		},
	}

	schemaSingle := &openapi.Schema{
		Example: map[string]any{"id": 99},
	}

	tests := []struct {
		name           string
		header         string
		strategy       string
		defaultExample string
		schema         *openapi.Schema
		expectedName   string
		expectedSource string
		headerMatched  bool
	}{
		{
			name:           "header match",
			header:         "alpha",
			strategy:       "header",
			schema:         schemaWithExamples,
			expectedName:   "alpha",
			expectedSource: "header",
			headerMatched:  true,
		},
		{
			name:           "header random",
			header:         "RANDOM",
			strategy:       "header",
			schema:         schemaWithExamples,
			expectedName:   "random",
			expectedSource: "header_random",
			headerMatched:  true,
		},
		{
			name:           "default fallback",
			header:         "missing",
			strategy:       "header",
			defaultExample: "beta",
			schema:         schemaWithExamples,
			expectedName:   "beta",
			expectedSource: "config_default",
			headerMatched:  false,
		},
		{
			name:           "first strategy",
			header:         "",
			strategy:       "first",
			schema:         schemaWithExamples,
			expectedName:   "alpha",
			expectedSource: "config_first",
			headerMatched:  false,
		},
		{
			name:           "random strategy",
			header:         "",
			strategy:       "random",
			schema:         schemaWithExamples,
			expectedName:   "random",
			expectedSource: "config_random",
			headerMatched:  false,
		},
		{
			name:           "header fallback to first",
			header:         "",
			strategy:       "header",
			schema:         schemaWithExamples,
			expectedName:   "alpha",
			expectedSource: "fallback_first",
			headerMatched:  false,
		},
		{
			name:           "single example",
			header:         "",
			strategy:       "random",
			schema:         schemaSingle,
			expectedName:   "",
			expectedSource: "single_example",
			headerMatched:  false,
		},
		{
			name:           "unknown strategy behaves as header",
			header:         "",
			strategy:       "custom",
			schema:         schemaWithExamples,
			expectedName:   "alpha",
			expectedSource: "fallback_first",
			headerMatched:  false,
		},
		{
			name:           "default missing falls back",
			header:         "",
			strategy:       "header",
			defaultExample: "gamma",
			schema:         schemaWithExamples,
			expectedName:   "alpha",
			expectedSource: "fallback_first",
			headerMatched:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveExampleSelection(tc.header, tc.schema, tc.strategy, tc.defaultExample)

			if result.Requested != tc.expectedName {
				t.Fatalf("expected example %q, got %q", tc.expectedName, result.Requested)
			}

			if result.Source != tc.expectedSource {
				t.Fatalf("expected source %q, got %q", tc.expectedSource, result.Source)
			}

			if result.HeaderMatched != tc.headerMatched {
				t.Fatalf("expected headerMatched %t, got %t", tc.headerMatched, result.HeaderMatched)
			}
		})
	}
}
