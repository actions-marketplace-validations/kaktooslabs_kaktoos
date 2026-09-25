package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kaktooslabs/kaktoos/internal/change"
	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
	"github.com/kaktooslabs/kaktoos/internal/discovery"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/impact"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/providers"
	"github.com/kaktooslabs/kaktoos/internal/reporter"
)

var (
	impactRepo    string
	impactCommit  string
	impactBase    string
	impactHead    string
	impactPR      int
	impactFormat  string
	impactWhy     bool
	impactVerify  bool
	impactEnv     string
	impactOpenAPI string
)

var impactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Analyze what a change touches and which workflows verify it",
	Long: "Analyze a change against the repository's engineering context: what " +
		"actually changed, what could be affected, who owns it, and which existing " +
		"Kaktoos workflows verify the affected behavior.\n\n" +
		"Potential impact is reachability, not proof — it never claims something is broken.",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch impactFormat {
		case "text", "json":
		default:
			fmt.Fprintf(cmd.ErrOrStderr(), "invalid --format %q (must be text or json)\n", impactFormat)
			os.Exit(2)
		}
		if (impactBase == "") != (impactHead == "") {
			fmt.Fprintln(cmd.ErrOrStderr(), "--base and --head must be used together")
			os.Exit(2)
		}

		src, err := impactSource()
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			os.Exit(2)
		}

		cs, err := src.Changes(context.Background(), impactRepo)
		if err != nil {
			if errors.Is(err, change.ErrProviderUnavailable) {
				fmt.Fprintf(cmd.ErrOrStderr(), "cannot read the change: %v\n", err)
				os.Exit(2)
			}
			return err
		}

		model, notes, err := discovery.Discover(impactRepo)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "cannot build engineering context: %v\n", err)
			os.Exit(2)
		}

		files := make([]impact.ChangedFile, 0, len(cs.Files))
		for _, f := range cs.Files {
			files = append(files, impact.ChangedFile{Path: f.Path, Status: f.Status})
		}
		result := impact.Analyze(model, files)
		for _, n := range notes {
			result.Notes = append(result.Notes, n.Source+": "+n.Message)
		}

		keys := providers.ExtractIssueKeys(cs.Branch, cs.Message)

		if impactFormat == "json" {
			if err := writeImpactJSON(cmd.OutOrStdout(), cs.Ref, result, keys); err != nil {
				return err
			}
		} else {
			writeImpactText(cmd.OutOrStdout(), cs.Ref, result, keys, impactWhy)
		}
		if impactVerify {
			return verifyPlan(cmd, result.Plan)
		}
		return nil
	},
}

// impactSource picks exactly one change source from the flags given.
func impactSource() (change.Source, error) {
	set := 0
	if impactCommit != "" {
		set++
	}
	if impactBase != "" {
		set++
	}
	if impactPR != 0 {
		set++
	}
	if set > 1 {
		return nil, errors.New("use only one of --commit, --base/--head, or --pr")
	}
	switch {
	case impactCommit != "":
		return change.Commit{SHA: impactCommit}, nil
	case impactBase != "":
		return change.Range{Base: impactBase, Head: impactHead}, nil
	case impactPR != 0:
		return change.PullRequest{Number: impactPR}, nil
	default:
		return change.WorkingTree{}, nil
	}
}

type impactJSON struct {
	Ref       string                  `json:"ref"`
	Changed   []contextmodel.Entity   `json:"changed"`
	Potential []impact.Affected       `json:"potential_impact"`
	Owners    []impact.Owner          `json:"owners"`
	Plan      impact.VerificationPlan `json:"verification"`
	WorkItems []string                `json:"related_work_keys,omitempty"`
	Notes     []string                `json:"notes,omitempty"`
}

func writeImpactJSON(w io.Writer, ref string, r impact.Impact, keys []string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(impactJSON{
		Ref:       ref,
		Changed:   r.Changed,
		Potential: r.Potential,
		Owners:    r.Owners,
		Plan:      r.Plan,
		WorkItems: keys,
		Notes:     r.Notes,
	})
}

func writeImpactText(w io.Writer, ref string, r impact.Impact, keys []string, why bool) {
	fmt.Fprintf(w, "Change: %s\n\n", ref)

	fmt.Fprintln(w, "Changed")
	if len(r.Changed) == 0 {
		fmt.Fprintln(w, "  (nothing)")
	}
	for _, e := range sortedEntities(r.Changed) {
		fmt.Fprintf(w, "  %s\n", e.Name)
	}

	fmt.Fprintln(w, "\nPotential impact (reachable from the change — not proof of a defect)")
	if len(r.Potential) == 0 {
		fmt.Fprintln(w, "  (none found)")
	}
	for _, a := range sortedAffected(r.Potential) {
		fmt.Fprintf(w, "  %-12s %s\n", a.Entity.Kind, a.Entity.Name)
		if why {
			for _, rel := range a.Path {
				inferred := ""
				if rel.Inferred {
					inferred = " [inferred]"
				}
				fmt.Fprintf(w, "               via %s: %s%s\n", rel.Kind, rel.Why, inferred)
			}
		}
	}

	fmt.Fprintln(w, "\nOwners")
	if len(r.Owners) == 0 {
		fmt.Fprintln(w, "  (none — no CODEOWNERS rule matched)")
	}
	for _, o := range sortedOwners(r.Owners) {
		fmt.Fprintf(w, "  %s (%s)\n", o.Name, strings.Join(o.Files, ", "))
	}

	fmt.Fprintln(w, "\nVerification (existing Kaktoos workflows)")
	if len(r.Plan.Workflows) == 0 {
		fmt.Fprintln(w, "  (no existing workflow covers the potentially affected behavior)")
	}
	for _, p := range sortedWorkflows(r.Plan.Workflows) {
		fmt.Fprintf(w, "  %s (%s)\n", p.Name, p.File)
	}

	if len(keys) > 0 {
		fmt.Fprintf(w, "\nRelated work (from branch/commit text)\n  %s\n", strings.Join(keys, ", "))
	}
	for _, n := range r.Notes {
		fmt.Fprintf(w, "\nnote: %s\n", n)
	}
}

// verifyPlan executes the scenarios the plan selected through the existing
// engine and reporter — no second execution path — and adopts run's exit code.
func verifyPlan(cmd *cobra.Command, plan impact.VerificationPlan) error {
	if len(plan.Workflows) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "\nnothing to verify: no existing workflow covers the potentially affected behavior")
		return nil
	}
	if impactOpenAPI == "" || impactEnv == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "--verify requires --openapi and --env")
		os.Exit(2)
	}

	env, err := config.Load(impactEnv)
	if err != nil {
		return withSuggestion(fmt.Errorf("loading environment: %w", err))
	}
	ops, err := openapi.Load(impactOpenAPI)
	if err != nil {
		return withSuggestion(fmt.Errorf("loading OpenAPI spec: %w", err))
	}

	paths := make([]string, 0, len(plan.Workflows))
	for _, p := range sortedWorkflows(plan.Workflows) {
		paths = append(paths, filepath.Join(impactRepo, p.File))
	}
	scenarios, err := loadScenarios(paths, nil)
	if err != nil {
		return withSuggestion(err)
	}

	client := httpclient.NewClient().HTTPClient()
	results := make([]engine.ExecutionResult, 0, len(scenarios))
	for _, scn := range scenarios {
		results = append(results, engine.RunScenario(engine.NewContext(env, ops), *scn, client))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nVerification result")
	reporter.Report(results, cmd.OutOrStdout())
	if code := reporter.ExitCode(results); code != 0 {
		os.Exit(code)
	}
	return nil
}

func sortedEntities(es []contextmodel.Entity) []contextmodel.Entity {
	out := append([]contextmodel.Entity{}, es...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedAffected(as []impact.Affected) []impact.Affected {
	out := append([]impact.Affected{}, as...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Entity.Kind != out[j].Entity.Kind {
			return out[i].Entity.Kind < out[j].Entity.Kind
		}
		return out[i].Entity.Name < out[j].Entity.Name
	})
	return out
}

func sortedOwners(os []impact.Owner) []impact.Owner {
	out := append([]impact.Owner{}, os...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedWorkflows(ps []impact.PlannedWorkflow) []impact.PlannedWorkflow {
	out := append([]impact.PlannedWorkflow{}, ps...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func init() {
	impactCmd.Flags().StringVar(&impactRepo, "repo", ".", "Repository root to analyze")
	impactCmd.Flags().StringVar(&impactCommit, "commit", "", "Analyze a single commit (SHA or revision)")
	impactCmd.Flags().StringVar(&impactBase, "base", "", "Base revision for a range comparison")
	impactCmd.Flags().StringVar(&impactHead, "head", "", "Head revision for a range comparison")
	impactCmd.Flags().IntVar(&impactPR, "pr", 0, "Analyze a GitHub pull request by number (requires the gh CLI)")
	impactCmd.Flags().StringVar(&impactFormat, "format", "text", "Output format: text or json")
	impactCmd.Flags().BoolVar(&impactWhy, "why", false, "Show the relation chain explaining each potential impact")
	impactCmd.Flags().BoolVar(&impactVerify, "verify", false, "Run the existing workflows the plan selected")
	impactCmd.Flags().StringVar(&impactOpenAPI, "openapi", "", "Path to the OpenAPI spec (required with --verify)")
	impactCmd.Flags().StringVar(&impactEnv, "env", "", "Path to the environment file (required with --verify)")
	rootCmd.AddCommand(impactCmd)
}
