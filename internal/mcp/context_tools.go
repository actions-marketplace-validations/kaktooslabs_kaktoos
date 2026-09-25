package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
	"github.com/kaktooslabs/kaktoos/internal/discovery"
	"github.com/kaktooslabs/kaktoos/internal/providers"
)

// EntitySummary is how an engineering entity is reported to an MCP client.
type EntitySummary struct {
	Kind  string            `json:"kind"`
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// RelationSummary explains one link in a chain. Inferred marks a relationship
// Kaktoos derived by convention rather than read from configuration.
type RelationSummary struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Kind     string `json:"kind"`
	Why      string `json:"why"`
	Inferred bool   `json:"inferred"`
}

func summarizeEntity(e contextmodel.Entity) EntitySummary {
	return EntitySummary{Kind: string(e.Kind), ID: e.ID, Name: e.Name, Attrs: e.Attrs}
}

func summarizeRelations(rels []contextmodel.Relation) []RelationSummary {
	out := make([]RelationSummary, 0, len(rels))
	for _, r := range rels {
		out = append(out, RelationSummary{
			From: r.From, To: r.To, Kind: string(r.Kind), Why: r.Why, Inferred: r.Inferred,
		})
	}
	return out
}

// loadModel builds the engineering context for a repository, defaulting to
// the current directory.
func loadModel(repo string) (*contextmodel.Model, []discovery.Note, error) {
	if strings.TrimSpace(repo) == "" {
		repo = "."
	}
	return discovery.Discover(repo)
}

func noteStrings(notes []discovery.Note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.Source+": "+n.Message)
	}
	return out
}

// ---------------------------------------------------------------------------
// get_related_context

type GetRelatedContextInput struct {
	Query string `json:"query" jsonschema:"what you are about to work on, in plain language (e.g. 'scheduled payment cancellation')"`
	Repo  string `json:"repo,omitempty" jsonschema:"repository root to analyze (default: current directory)"`
}

type GetRelatedContextOutput struct {
	Query     string          `json:"query"`
	Matched   []EntitySummary `json:"matched"`
	Related   []EntitySummary `json:"related"`
	Owners    []string        `json:"owners,omitempty"`
	Workflows []EntitySummary `json:"workflows,omitempty"`
	// Explanations records why each matched entity was returned.
	Explanations []RelationSummary `json:"explanations,omitempty"`
	Notes        []string          `json:"notes,omitempty"`
}

// GetRelatedContext returns the parts of the existing system that a plain
// language description touches. It does no inference about intent: it
// tokenizes the query and matches those tokens against entity names, paths
// and operation paths already discovered from the repository. Nothing is
// returned that is not backed by a file in the repo.
func GetRelatedContext(_ context.Context, _ *mcpsdk.CallToolRequest, in GetRelatedContextInput) (*mcpsdk.CallToolResult, GetRelatedContextOutput, error) {
	if strings.TrimSpace(in.Query) == "" {
		return errorResult("query is required"), GetRelatedContextOutput{}, nil
	}
	model, notes, err := loadModel(in.Repo)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot build engineering context: %v", err)), GetRelatedContextOutput{}, nil
	}

	terms := tokenize(in.Query)
	out := GetRelatedContextOutput{
		Query:   in.Query,
		Matched: []EntitySummary{},
		Related: []EntitySummary{},
		Notes:   noteStrings(notes),
	}

	var seeds []string
	for _, e := range sortEntities(model.Entities()) {
		if !entityMatches(e, terms) {
			continue
		}
		out.Matched = append(out.Matched, summarizeEntity(e))
		seeds = append(seeds, e.ID)
	}

	if len(seeds) == 0 {
		out.Notes = append(out.Notes, "nothing in this repository matched the query; no relationships were inferred")
		return nil, out, nil
	}

	seen := map[string]bool{}
	for id, path := range model.Reachable(seeds, 3) {
		e, ok := model.Get(id)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		switch e.Kind {
		case contextmodel.KindOwner:
			out.Owners = append(out.Owners, e.Name)
		case contextmodel.KindWorkflow:
			out.Workflows = append(out.Workflows, summarizeEntity(e))
			out.Related = append(out.Related, summarizeEntity(e))
		default:
			out.Related = append(out.Related, summarizeEntity(e))
		}
		out.Explanations = append(out.Explanations, summarizeRelations(path)...)
	}
	sort.Strings(out.Owners)
	sortSummaries(out.Related)
	sortSummaries(out.Workflows)
	return nil, out, nil
}

// ---------------------------------------------------------------------------
// get_dependencies

type GetDependenciesInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity id such as 'service:payment-service', 'file:payment-service/fee.go' or 'operation:POST /payments'"`
	Repo     string `json:"repo,omitempty" jsonschema:"repository root to analyze (default: current directory)"`
}

type GetDependenciesOutput struct {
	Entity     EntitySummary   `json:"entity"`
	DependsOn  []EntitySummary `json:"depends_on"`
	ConsumedBy []EntitySummary `json:"consumed_by"`
	Exposes    []EntitySummary `json:"exposes"`
	Notes      []string        `json:"notes,omitempty"`
}

// GetDependencies reports the direct dependency edges around one entity.
func GetDependencies(_ context.Context, _ *mcpsdk.CallToolRequest, in GetDependenciesInput) (*mcpsdk.CallToolResult, GetDependenciesOutput, error) {
	if strings.TrimSpace(in.EntityID) == "" {
		return errorResult("entity_id is required"), GetDependenciesOutput{}, nil
	}
	model, notes, err := loadModel(in.Repo)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot build engineering context: %v", err)), GetDependenciesOutput{}, nil
	}
	e, ok := model.Get(in.EntityID)
	if !ok {
		return errorResult(fmt.Sprintf("unknown entity %q — call get_related_context first to discover valid ids", in.EntityID)), GetDependenciesOutput{}, nil
	}

	out := GetDependenciesOutput{
		Entity:     summarizeEntity(e),
		DependsOn:  []EntitySummary{},
		ConsumedBy: []EntitySummary{},
		Exposes:    []EntitySummary{},
		Notes:      noteStrings(notes),
	}
	for _, d := range model.Related(in.EntityID, contextmodel.RelDependsOn) {
		out.DependsOn = append(out.DependsOn, summarizeEntity(d))
	}
	for _, x := range model.Related(in.EntityID, contextmodel.RelExposes) {
		out.Exposes = append(out.Exposes, summarizeEntity(x))
	}
	// Consumed-by is the reverse edge, found by scanning relations.
	for _, r := range model.Relations() {
		if r.To == in.EntityID && r.Kind == contextmodel.RelDependsOn {
			if from, ok := model.Get(r.From); ok {
				out.ConsumedBy = append(out.ConsumedBy, summarizeEntity(from))
			}
		}
	}
	sortSummaries(out.DependsOn)
	sortSummaries(out.ConsumedBy)
	sortSummaries(out.Exposes)
	return nil, out, nil
}

// ---------------------------------------------------------------------------
// get_owners

type GetOwnersInput struct {
	Paths []string `json:"paths" jsonschema:"repository-relative file paths to look up"`
	Repo  string   `json:"repo,omitempty" jsonschema:"repository root to analyze (default: current directory)"`
}

type PathOwners struct {
	Path   string   `json:"path"`
	Owners []string `json:"owners"`
}

type GetOwnersOutput struct {
	// Source names where ownership came from, so a caller can weigh it.
	Source string       `json:"source"`
	Result []PathOwners `json:"result"`
	Note   string       `json:"note"`
}

// GetOwners resolves CODEOWNERS ownership. Ownership is contextual: a path no
// rule matches returns no owners rather than a guess.
func GetOwners(_ context.Context, _ *mcpsdk.CallToolRequest, in GetOwnersInput) (*mcpsdk.CallToolResult, GetOwnersOutput, error) {
	if len(in.Paths) == 0 {
		return errorResult("paths is required"), GetOwnersOutput{}, nil
	}
	repo := in.Repo
	if strings.TrimSpace(repo) == "" {
		repo = "."
	}
	owners, err := discovery.LoadCodeowners(repo)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot read CODEOWNERS: %v", err)), GetOwnersOutput{}, nil
	}
	out := GetOwnersOutput{
		Source: "CODEOWNERS",
		Result: []PathOwners{},
		Note:   "ownership is contextual information from CODEOWNERS, not a guarantee of who is responsible",
	}
	for _, p := range in.Paths {
		o := owners.Owners(p)
		if o == nil {
			o = []string{}
		}
		out.Result = append(out.Result, PathOwners{Path: p, Owners: o})
	}
	return nil, out, nil
}

// ---------------------------------------------------------------------------
// get_related_work / get_related_documents

type GetRelatedWorkInput struct {
	Text  string   `json:"text,omitempty" jsonschema:"free text to scan for issue keys, e.g. a branch name or commit message"`
	Keys  []string `json:"keys,omitempty" jsonschema:"issue keys you already know"`
	Terms []string `json:"terms,omitempty" jsonschema:"search terms for a configured work-item provider"`
}

type GetRelatedWorkOutput struct {
	Provider    string               `json:"provider"`
	Available   bool                 `json:"available"`
	Items       []providers.WorkItem `json:"items"`
	Unavailable string               `json:"unavailable_reason,omitempty"`
}

// GetRelatedWork reports work items related to a change. Today the only
// provider reads issue keys straight out of the text it is given; a Jira
// provider can be added behind the same interface without touching callers.
func GetRelatedWork(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetRelatedWorkInput) (*mcpsdk.CallToolResult, GetRelatedWorkOutput, error) {
	keys := append([]string{}, in.Keys...)
	keys = append(keys, providers.ExtractIssueKeys(in.Text)...)

	p := workItemProvider()
	items, err := p.Find(ctx, keys, in.Terms)
	out := GetRelatedWorkOutput{Provider: p.Name(), Available: err == nil, Items: items}
	if err != nil {
		out.Items = []providers.WorkItem{}
		out.Unavailable = err.Error()
	}
	if out.Items == nil {
		out.Items = []providers.WorkItem{}
	}
	return nil, out, nil
}

type GetRelatedDocumentsInput struct {
	Terms []string `json:"terms" jsonschema:"search terms for a configured documentation provider"`
}

type GetRelatedDocumentsOutput struct {
	Provider    string               `json:"provider"`
	Available   bool                 `json:"available"`
	Documents   []providers.Document `json:"documents"`
	Unavailable string               `json:"unavailable_reason,omitempty"`
}

// GetRelatedDocuments reports related documentation. No document provider
// ships yet, so this reports cleanly that the source is unavailable rather
// than implying no documentation exists.
func GetRelatedDocuments(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetRelatedDocumentsInput) (*mcpsdk.CallToolResult, GetRelatedDocumentsOutput, error) {
	p := documentProvider()
	docs, err := p.Find(ctx, in.Terms)
	out := GetRelatedDocumentsOutput{Provider: p.Name(), Available: err == nil, Documents: docs}
	if err != nil {
		out.Documents = []providers.Document{}
		out.Unavailable = err.Error()
	}
	if out.Documents == nil {
		out.Documents = []providers.Document{}
	}
	return nil, out, nil
}

// workItemProvider and documentProvider are the single place providers are
// selected. Swapping in a Jira or Confluence provider happens here.
func workItemProvider() providers.WorkItemProvider { return providers.LocalWorkItems{} }

func documentProvider() providers.DocumentProvider {
	return providers.UnavailableDocuments{Reason: "no documentation provider is configured"}
}

// ---------------------------------------------------------------------------
// helpers

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "for": true, "of": true,
	"and": true, "or": true, "in": true, "on": true, "add": true, "new": true,
	"change": true, "update": true, "support": true, "feature": true,
}

// tokenize splits a natural-language query into lowercase terms, dropping
// stop words and very short fragments. No stemming, no synonyms — matching
// stays predictable and explainable.
func tokenize(q string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(f) < 3 || stopWords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// entityMatches reports whether any query term appears in the entity's name
// or attributes.
func entityMatches(e contextmodel.Entity, terms []string) bool {
	hay := strings.ToLower(e.Name)
	for _, v := range e.Attrs {
		hay += " " + strings.ToLower(v)
	}
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

func sortEntities(es []contextmodel.Entity) []contextmodel.Entity {
	out := append([]contextmodel.Entity{}, es...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortSummaries(ss []EntitySummary) {
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].Kind != ss[j].Kind {
			return ss[i].Kind < ss[j].Kind
		}
		return ss[i].Name < ss[j].Name
	})
}
