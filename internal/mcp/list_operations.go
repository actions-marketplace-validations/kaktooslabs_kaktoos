package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kaktooslabs/kaktoos/internal/openapi"
)

type ListOperationsInput struct {
	OpenAPIPath string `json:"openapi_path" jsonschema:"path to the OpenAPI specification file"`
	Filter      string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring matched against operationId, path or tag"`
}

type ParameterSummary struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

type OperationSummary struct {
	OperationID string             `json:"operation_id"`
	Method      string             `json:"method"`
	Path        string             `json:"path"`
	Description string             `json:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Parameters  []ParameterSummary `json:"parameters,omitempty"`
	HasRequest  bool               `json:"has_request_body"`
	Responses   []string           `json:"responses,omitempty"`
}

type ListOperationsOutput struct {
	Operations []OperationSummary `json:"operations"`
}

// ListOperations discovers the operations declared in an OpenAPI spec.
// It reuses openapi.Load — no separate spec parsing.
func ListOperations(_ context.Context, _ *mcpsdk.CallToolRequest, in ListOperationsInput) (*mcpsdk.CallToolResult, ListOperationsOutput, error) {
	if strings.TrimSpace(in.OpenAPIPath) == "" {
		return errorResult("openapi_path is required"), ListOperationsOutput{}, nil
	}
	ops, err := openapi.Load(in.OpenAPIPath)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot load OpenAPI spec: %v", err)), ListOperationsOutput{}, nil
	}

	filter := strings.ToLower(in.Filter)
	out := ListOperationsOutput{Operations: []OperationSummary{}}
	for path, inner := range ops {
		for method, op := range inner {
			if filter != "" && !matchesFilter(op, path, filter) {
				continue
			}
			out.Operations = append(out.Operations, summarize(op, path, method))
		}
	}
	sort.Slice(out.Operations, func(i, j int) bool {
		if out.Operations[i].Path != out.Operations[j].Path {
			return out.Operations[i].Path < out.Operations[j].Path
		}
		return out.Operations[i].Method < out.Operations[j].Method
	})
	return nil, out, nil
}

func matchesFilter(op openapi.Operation, path, filter string) bool {
	if strings.Contains(strings.ToLower(op.Name), filter) || strings.Contains(strings.ToLower(path), filter) {
		return true
	}
	for _, tag := range op.Tags {
		if strings.Contains(strings.ToLower(tag), filter) {
			return true
		}
	}
	return false
}

func summarize(op openapi.Operation, path, method string) OperationSummary {
	s := OperationSummary{
		OperationID: op.Name,
		Method:      method,
		Path:        path,
		Description: op.Description,
		Tags:        op.Tags,
		HasRequest:  op.RequestBody != nil,
	}
	for _, p := range op.Parameters {
		s.Parameters = append(s.Parameters, ParameterSummary{Name: p.Name, In: p.In, Required: p.Required})
	}
	for code := range op.Responses {
		s.Responses = append(s.Responses, code)
	}
	sort.Strings(s.Responses)
	return s
}
