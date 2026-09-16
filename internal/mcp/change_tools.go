package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kaktooslabs/kaktoos/internal/change"
	"github.com/kaktooslabs/kaktoos/internal/impact"
	"github.com/kaktooslabs/kaktoos/internal/providers"
)

// AnalyzeChangeInput selects which change to analyze. Exactly one selector
// may be given; with none, the working tree is analyzed.
type AnalyzeChangeInput struct {
	Repo   string `json:"repo,omitempty" jsonschema:"repository root (default: current directory)"`
	Commit string `json:"commit,omitempty" jsonschema:"a commit SHA or revision to analyze"`
	Base   string `json:"base,omitempty" jsonschema:"base revision for a range comparison (use with head)"`
	Head   string `json:"head,omitempty" jsonschema:"head revision for a range comparison (use with base)"`
	PR     int    `json:"pr,omitempty" jsonschema:"GitHub pull request number (requires the gh CLI)"`
}

// AffectedSummary is one potentially affected entity and the chain explaining
// why it was reached.
type AffectedSummary struct {
	Entity EntitySummary     `json:"entity"`
	Path   []RelationSummary `json:"path"`
}

type PlannedWorkflowSummary struct {
	Name   string            `json:"name"`
	File   string            `json:"file"`
	Reason []RelationSummary `json:"reason"`
}

type AnalyzeChangeOutput struct {
	Ref string `json:"ref"`
	// Changed is fact: these files were modified.
	Changed []EntitySummary `json:"changed"`
	// Potential is reachability, not proof — never a claim of a defect.
	Potential       []AffectedSummary        `json:"potential_impact"`
	Owners          []impact.Owner           `json:"owners"`
	Workflows       []PlannedWorkflowSummary `json:"verification_workflows"`
	RelatedWorkKeys []string                 `json:"related_work_keys,omitempty"`
	Notes           []string                 `json:"notes,omitempty"`
	Disclaimer      string                   `json:"disclaimer"`
}

const impactDisclaimer = "Changed lists what the diff actually touched. Potential impact is what is reachable " +
	"from those files through declared and inferred relationships — it indicates what to verify, not what is broken."

// AnalyzeChange reports what a change touched and what it could affect.
func AnalyzeChange(ctx context.Context, _ *mcpsdk.CallToolRequest, in AnalyzeChangeInput) (*mcpsdk.CallToolResult, AnalyzeChangeOutput, error) {
	result, errResult := analyzeChange(ctx, in)
	if errResult != nil {
		return errResult, AnalyzeChangeOutput{}, nil
	}
	return nil, result, nil
}

// GetVerificationPlanInput mirrors AnalyzeChangeInput: a plan is derived from
// the same analysis.
type GetVerificationPlanInput = AnalyzeChangeInput

type GetVerificationPlanOutput struct {
	Ref       string                   `json:"ref"`
	Workflows []PlannedWorkflowSummary `json:"workflows"`
	Notes     []string                 `json:"notes,omitempty"`
	Guidance  string                   `json:"guidance"`
}

// GetVerificationPlan returns the existing Kaktoos workflows that verify the
// potentially affected behavior. It only ever selects scenarios that already
// exist in the repository; it never generates one.
func GetVerificationPlan(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetVerificationPlanInput) (*mcpsdk.CallToolResult, GetVerificationPlanOutput, error) {
	result, errResult := analyzeChange(ctx, in)
	if errResult != nil {
		return errResult, GetVerificationPlanOutput{}, nil
	}
	out := GetVerificationPlanOutput{
		Ref:       result.Ref,
		Workflows: result.Workflows,
		Notes:     result.Notes,
		Guidance:  "Run these with run_verification. Only existing scenarios are selected; none are generated.",
	}
	if len(out.Workflows) == 0 {
		out.Notes = append(out.Notes, "no existing workflow covers the potentially affected behavior")
	}
	return nil, out, nil
}

// analyzeChange is the shared body of analyze_change and
// get_verification_plan.
func analyzeChange(ctx context.Context, in AnalyzeChangeInput) (AnalyzeChangeOutput, *mcpsdk.CallToolResult) {
	repo := in.Repo
	if strings.TrimSpace(repo) == "" {
		repo = "."
	}
	src, err := changeSource(in)
	if err != nil {
		return AnalyzeChangeOutput{}, errorResult(err.Error())
	}
	cs, err := src.Changes(ctx, repo)
	if err != nil {
		if errors.Is(err, change.ErrProviderUnavailable) {
			return AnalyzeChangeOutput{}, errorResult(fmt.Sprintf("change source unavailable: %v", err))
		}
		return AnalyzeChangeOutput{}, errorResult(fmt.Sprintf("cannot read the change: %v", err))
	}
	model, notes, err := loadModel(repo)
	if err != nil {
		return AnalyzeChangeOutput{}, errorResult(fmt.Sprintf("cannot build engineering context: %v", err))
	}

	files := make([]impact.ChangedFile, 0, len(cs.Files))
	for _, f := range cs.Files {
		files = append(files, impact.ChangedFile{Path: f.Path, Status: f.Status})
	}
	res := impact.Analyze(model, files)

	out := AnalyzeChangeOutput{
		Ref:             cs.Ref,
		Changed:         []EntitySummary{},
		Potential:       []AffectedSummary{},
		Owners:          res.Owners,
		Workflows:       []PlannedWorkflowSummary{},
		RelatedWorkKeys: providers.ExtractIssueKeys(cs.Branch, cs.Message),
		Notes:           append(noteStrings(notes), res.Notes...),
		Disclaimer:      impactDisclaimer,
	}
	for _, e := range res.Changed {
		out.Changed = append(out.Changed, summarizeEntity(e))
	}
	for _, a := range res.Potential {
		out.Potential = append(out.Potential, AffectedSummary{
			Entity: summarizeEntity(a.Entity),
			Path:   summarizeRelations(a.Path),
		})
	}
	for _, w := range res.Plan.Workflows {
		out.Workflows = append(out.Workflows, PlannedWorkflowSummary{
			Name: w.Name, File: w.File, Reason: summarizeRelations(w.Reason),
		})
	}
	if out.Owners == nil {
		out.Owners = []impact.Owner{}
	}
	return out, nil
}

func changeSource(in AnalyzeChangeInput) (change.Source, error) {
	set := 0
	if in.Commit != "" {
		set++
	}
	if in.Base != "" || in.Head != "" {
		set++
	}
	if in.PR != 0 {
		set++
	}
	if set > 1 {
		return nil, errors.New("give only one of commit, base/head, or pr")
	}
	switch {
	case in.Commit != "":
		return change.Commit{SHA: in.Commit}, nil
	case in.Base != "" || in.Head != "":
		if in.Base == "" || in.Head == "" {
			return nil, errors.New("base and head must be given together")
		}
		return change.Range{Base: in.Base, Head: in.Head}, nil
	case in.PR != 0:
		return change.PullRequest{Number: in.PR}, nil
	default:
		return change.WorkingTree{}, nil
	}
}
