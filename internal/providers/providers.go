// Package providers defines how Kaktoos looks up related work items and
// documents. These are interfaces on purpose: a Jira or Confluence provider
// can be added later without touching any core package. Nothing here calls
// out to a real Jira or Confluence — that integration is future work.
package providers

import (
	"context"
	"errors"
	"regexp"
)

// ErrProviderUnavailable means the provider is not configured. Callers must
// report this as "unavailable" for that source, not fail the whole query.
var ErrProviderUnavailable = errors.New("provider unavailable")

// WorkItem is one related issue/ticket, source-agnostic.
type WorkItem struct {
	Key    string `json:"key"`
	Title  string `json:"title,omitempty"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

// WorkItemProvider finds work items related to a change by issue key or text.
type WorkItemProvider interface {
	Name() string
	Find(ctx context.Context, keys []string, terms []string) ([]WorkItem, error)
}

// Document is one related piece of documentation, source-agnostic.
type Document struct {
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
	Source string `json:"source,omitempty"`
}

// DocumentProvider finds documents related to a set of search terms.
type DocumentProvider interface {
	Name() string
	Find(ctx context.Context, terms []string) ([]Document, error)
}

// issueKeyPattern matches typical tracker issue keys, e.g. "PROJ-42".
var issueKeyPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

// ExtractIssueKeys pulls issue keys out of free text (a branch name, a
// commit message, a PR title/body) with plain regex matching — no API call,
// no guessing at a project the text doesn't mention.
func ExtractIssueKeys(texts ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range texts {
		for _, k := range issueKeyPattern.FindAllString(t, -1) {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}

// LocalWorkItems reports only the issue keys it can read directly out of the
// text it's given (branch name, commit/PR message) — no Jira credentials, no
// network call. It never fails: an unmatched key search simply returns none.
type LocalWorkItems struct{}

func (LocalWorkItems) Name() string { return "local" }

func (LocalWorkItems) Find(_ context.Context, keys []string, _ []string) ([]WorkItem, error) {
	items := make([]WorkItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, WorkItem{Key: k})
	}
	return items, nil
}

// UnavailableWorkItems is the default when no work-item provider (e.g. Jira)
// is configured: it reports unavailable rather than pretending there is
// nothing related.
type UnavailableWorkItems struct{ Reason string }

func (u UnavailableWorkItems) Name() string { return "unavailable" }

func (u UnavailableWorkItems) Find(context.Context, []string, []string) ([]WorkItem, error) {
	return nil, unavailableErr(u.Reason)
}

// UnavailableDocuments is the default when no document provider (e.g.
// Confluence) is configured.
type UnavailableDocuments struct{ Reason string }

func (u UnavailableDocuments) Name() string { return "unavailable" }

func (u UnavailableDocuments) Find(context.Context, []string) ([]Document, error) {
	return nil, unavailableErr(u.Reason)
}

func unavailableErr(reason string) error {
	if reason == "" {
		reason = "no provider configured"
	}
	return errors.Join(ErrProviderUnavailable, errors.New(reason))
}
