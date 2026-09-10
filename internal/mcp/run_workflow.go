package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
)

type RunWorkflowInput struct {
	ScenarioYAML string `json:"scenario_yaml" jsonschema:"the scenario to execute as inline YAML"`
	OpenAPIPath  string `json:"openapi_path" jsonschema:"path to the OpenAPI specification file"`
	Environment  string `json:"environment,omitempty" jsonschema:"inline environment YAML (base_url etc); mutually exclusive with env_path"`
	EnvPath      string `json:"env_path,omitempty" jsonschema:"path to an environment file; used when environment is not given"`
	SchemaMode   string `json:"schema_mode,omitempty" jsonschema:"off, warn or strict; defaults to strict for agent verification"`
}

type SchemaViolation struct {
	Kind    string `json:"kind" jsonschema:"one of status_undeclared, content_type_mismatch, required_field_missing, schema_mismatch"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type AssertionFailure struct {
	Type     string `json:"type"`
	Path     string `json:"path,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Error    string `json:"error,omitempty"`
}

type StepOutcome struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code,omitempty"`
	// Error is set for transport/infrastructure problems (no StatusCode) or
	// as the headline reason for a failure.
	Error             string             `json:"error,omitempty"`
	AssertionFailures []AssertionFailure `json:"assertion_failures,omitempty"`
	SchemaViolations  []SchemaViolation  `json:"schema_violations,omitempty"`
	UndeclaredFields  []string           `json:"undeclared_fields,omitempty" jsonschema:"informational only; never causes failure"`
}

type RunWorkflowOutput struct {
	Passed bool          `json:"passed"`
	Status string        `json:"status"`
	Steps  []StepOutcome `json:"steps"`
}

// RunWorkflow executes a scenario through engine.RunScenario. It is a pure
// adapter: input parsing in, struct mapping out, no execution logic of its own.
func RunWorkflow(_ context.Context, _ *mcpsdk.CallToolRequest, in RunWorkflowInput) (*mcpsdk.CallToolResult, RunWorkflowOutput, error) {
	if strings.TrimSpace(in.ScenarioYAML) == "" {
		return errorResult("scenario_yaml is required"), RunWorkflowOutput{}, nil
	}
	if strings.TrimSpace(in.OpenAPIPath) == "" {
		return errorResult("openapi_path is required"), RunWorkflowOutput{}, nil
	}

	mode := schema.Mode(in.SchemaMode)
	if in.SchemaMode == "" {
		mode = schema.Strict // agent verification wants contract enforcement by default
	}
	switch mode {
	case schema.Off, schema.Warn, schema.Strict:
	default:
		return errorResult(fmt.Sprintf("invalid schema_mode %q: must be off, warn or strict", in.SchemaMode)), RunWorkflowOutput{}, nil
	}

	ops, err := openapi.Load(in.OpenAPIPath)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot load OpenAPI spec: %v", err)), RunWorkflowOutput{}, nil
	}

	var env config.Environment
	switch {
	case strings.TrimSpace(in.Environment) != "":
		env, err = config.LoadBytes([]byte(in.Environment))
	case strings.TrimSpace(in.EnvPath) != "":
		env, err = config.Load(in.EnvPath)
	default:
		return errorResult("one of environment or env_path is required"), RunWorkflowOutput{}, nil
	}
	if err != nil {
		return errorResult(fmt.Sprintf("cannot load environment: %v", err)), RunWorkflowOutput{}, nil
	}

	scn, err := scenario.Load([]byte(in.ScenarioYAML))
	if err != nil {
		return errorResult(fmt.Sprintf("invalid scenario: %v", err)), RunWorkflowOutput{}, nil
	}

	ctx := engine.NewContext(env, ops)
	ctx.SchemaMode = mode
	result := engine.RunScenario(ctx, *scn, httpclient.NewClient().HTTPClient())

	return nil, toOutput(result), nil
}

func toOutput(r engine.ExecutionResult) RunWorkflowOutput {
	out := RunWorkflowOutput{Passed: r.Passed, Status: string(r.Status), Steps: make([]StepOutcome, 0, len(r.Steps))}
	for _, s := range r.Steps {
		step := StepOutcome{
			Name:             s.Name,
			Status:           string(s.Status),
			StatusCode:       s.StatusCode,
			Error:            s.Error,
			UndeclaredFields: s.UndeclaredFields,
		}
		for _, a := range s.Assertions {
			if a.Passed {
				continue
			}
			step.AssertionFailures = append(step.AssertionFailures, AssertionFailure{
				Type:     a.Type,
				Path:     a.Path,
				Expected: fmt.Sprintf("%v", a.Expected),
				Actual:   fmt.Sprintf("%v", a.Actual),
				Error:    a.Error,
			})
		}
		for _, v := range s.SchemaViolations {
			step.SchemaViolations = append(step.SchemaViolations, SchemaViolation{Kind: v.Kind, Path: v.Path, Message: v.Message})
		}
		out.Steps = append(out.Steps, step)
	}
	return out
}
