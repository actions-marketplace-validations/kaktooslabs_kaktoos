package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

type ValidateScenarioInput struct {
	ScenarioYAML string `json:"scenario_yaml" jsonschema:"the scenario definition as inline YAML"`
	OpenAPIPath  string `json:"openapi_path,omitempty" jsonschema:"optional OpenAPI spec; when given, every step operation must resolve against it"`
}

type ValidateScenarioOutput struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ValidateScenario parses a scenario from memory and, if an OpenAPI spec is
// given, checks that every step operation resolves. It performs no network
// calls and never executes the scenario.
func ValidateScenario(_ context.Context, _ *mcpsdk.CallToolRequest, in ValidateScenarioInput) (*mcpsdk.CallToolResult, ValidateScenarioOutput, error) {
	if strings.TrimSpace(in.ScenarioYAML) == "" {
		return errorResult("scenario_yaml is required"), ValidateScenarioOutput{}, nil
	}

	scn, err := scenario.Load([]byte(in.ScenarioYAML))
	if err != nil {
		return nil, ValidateScenarioOutput{Valid: false, Errors: []string{err.Error()}}, nil
	}
	if in.OpenAPIPath == "" {
		return nil, ValidateScenarioOutput{Valid: true}, nil
	}

	ops, err := openapi.Load(in.OpenAPIPath)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot load OpenAPI spec: %v", err)), ValidateScenarioOutput{}, nil
	}

	var errs []string
	for _, step := range scn.Steps {
		if step.Operation == "" {
			continue // condition-only steps declare no operation
		}
		if _, _, _, found := openapi.FindOperation(ops, step.Operation); !found {
			errs = append(errs, fmt.Sprintf("step %q: operation %q not found in OpenAPI spec", step.Name, step.Operation))
		}
	}
	return nil, ValidateScenarioOutput{Valid: len(errs) == 0, Errors: errs}, nil
}
