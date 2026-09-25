package mcp

import (
	"context"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type ProposeChangeInput struct {
	Description string `json:"description" jsonschema:"the change you are considering, in plain language"`
	Repo        string `json:"repo,omitempty" jsonschema:"repository root to analyze (default: current directory)"`
}

// ProposeChangeOutput is grounded context for a proposed change. It contains
// no design: the agent remains responsible for deciding what to build.
type ProposeChangeOutput struct {
	Description string `json:"description"`
	// Existing is what already exists in the system related to the description.
	Existing GetRelatedContextOutput `json:"existing_context"`
	// RecommendedVerification names existing workflows covering the area.
	RecommendedVerification []EntitySummary `json:"recommended_verification"`
	Considerations          []string        `json:"considerations,omitempty"`
	Guidance                string          `json:"guidance"`
}

// ProposeChange returns the existing system context for a proposed change.
// It is deliberately a thin envelope over get_related_context: Kaktoos
// supplies grounded facts about what exists, never a design or a requirement
// it inferred on its own.
func ProposeChange(ctx context.Context, req *mcpsdk.CallToolRequest, in ProposeChangeInput) (*mcpsdk.CallToolResult, ProposeChangeOutput, error) {
	if strings.TrimSpace(in.Description) == "" {
		return errorResult("description is required"), ProposeChangeOutput{}, nil
	}
	errRes, related, err := GetRelatedContext(ctx, req, GetRelatedContextInput{Query: in.Description, Repo: in.Repo})
	if errRes != nil || err != nil {
		return errRes, ProposeChangeOutput{}, err
	}

	out := ProposeChangeOutput{
		Description:             in.Description,
		Existing:                related,
		RecommendedVerification: related.Workflows,
		Guidance: "This is existing system context only — no design is implied. " +
			"After implementing, run analyze_change and then run_verification on the workflows above.",
	}
	if len(related.Matched) == 0 {
		out.Considerations = append(out.Considerations,
			"nothing in this repository matched the description; treat this as greenfield rather than assuming hidden dependencies")
	}
	if len(related.Workflows) == 0 && len(related.Matched) > 0 {
		out.Considerations = append(out.Considerations,
			"no existing Kaktoos workflow covers this area, so a change here would land unverified")
	}
	for _, e := range related.Related {
		if e.Kind == "service" {
			out.Considerations = append(out.Considerations,
				"service "+e.Name+" is connected to this area — check get_dependencies before changing shared behavior")
		}
	}
	return nil, out, nil
}
