package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunVerificationInput runs an existing scenario file — the one a
// verification plan named — rather than inline YAML.
type RunVerificationInput struct {
	ScenarioPath string `json:"scenario_path" jsonschema:"path to an existing scenario file, as returned by get_verification_plan"`
	OpenAPIPath  string `json:"openapi_path" jsonschema:"path to the OpenAPI specification file"`
	Repo         string `json:"repo,omitempty" jsonschema:"repository root that scenario_path and openapi_path are relative to (default: current directory)"`
	Environment  string `json:"environment,omitempty" jsonschema:"inline environment YAML (base_url etc); mutually exclusive with env_path"`
	EnvPath      string `json:"env_path,omitempty" jsonschema:"path to an environment file; used when environment is not given"`
	SchemaMode   string `json:"schema_mode,omitempty" jsonschema:"off, warn or strict; defaults to strict"`
}

type RunVerificationOutput struct {
	Scenario string `json:"scenario"`
	RunWorkflowOutput
}

// RunVerification executes an existing scenario through the same path as
// run_workflow — and therefore through engine.RunScenario. It adds no
// execution logic: it only resolves the scenario file and delegates.
func RunVerification(ctx context.Context, req *mcpsdk.CallToolRequest, in RunVerificationInput) (*mcpsdk.CallToolResult, RunVerificationOutput, error) {
	if strings.TrimSpace(in.ScenarioPath) == "" {
		return errorResult("scenario_path is required"), RunVerificationOutput{}, nil
	}
	repo := in.Repo
	if strings.TrimSpace(repo) == "" {
		repo = "."
	}

	scenarioPath := resolveUnderRepo(repo, in.ScenarioPath)
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot read scenario %s: %v", in.ScenarioPath, err)), RunVerificationOutput{}, nil
	}

	result, out, err := RunWorkflow(ctx, req, RunWorkflowInput{
		ScenarioYAML: string(data),
		OpenAPIPath:  resolveUnderRepo(repo, in.OpenAPIPath),
		Environment:  in.Environment,
		EnvPath:      in.EnvPath,
		SchemaMode:   in.SchemaMode,
	})
	if result != nil || err != nil {
		return result, RunVerificationOutput{}, err
	}
	return nil, RunVerificationOutput{Scenario: in.ScenarioPath, RunWorkflowOutput: out}, nil
}

// resolveUnderRepo joins a repo-relative path with the repo root, leaving
// absolute paths alone.
func resolveUnderRepo(repo, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(repo, path)
}
