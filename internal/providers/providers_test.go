package providers

import (
	"context"
	"errors"
	"sort"
	"testing"
)

func TestExtractIssueKeys(t *testing.T) {
	got := ExtractIssueKeys("feature/PROJ-42-cancel-payments", "fixes PROJ-42, see also OPS-7 and OPS-7 again")
	sort.Strings(got)
	want := []string{"OPS-7", "PROJ-42"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractIssueKeysNoneFound(t *testing.T) {
	if got := ExtractIssueKeys("just a normal commit message"); len(got) != 0 {
		t.Fatalf("expected no keys, got %v", got)
	}
}

func TestLocalWorkItemsNeverFails(t *testing.T) {
	items, err := LocalWorkItems{}.Find(context.Background(), []string{"PROJ-42"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Key != "PROJ-42" {
		t.Fatalf("items: %+v", items)
	}
}

func TestUnavailableWorkItemsReportsUnavailable(t *testing.T) {
	_, err := UnavailableWorkItems{Reason: "jira not configured"}.Find(context.Background(), nil, nil)
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func TestUnavailableDocumentsReportsUnavailable(t *testing.T) {
	_, err := UnavailableDocuments{}.Find(context.Background(), nil)
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}
