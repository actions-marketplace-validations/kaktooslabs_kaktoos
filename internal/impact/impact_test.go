package impact

import (
	"sort"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
	"github.com/kaktooslabs/kaktoos/internal/discovery"
)

const fixture = "../discovery/testdata/monorepo"

func model(t *testing.T) *contextmodel.Model {
	t.Helper()
	m, _, err := discovery.Discover(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func kindNames(imp Impact, kind contextmodel.Kind) []string {
	var out []string
	for _, a := range imp.Potential {
		if a.Entity.Kind == kind {
			out = append(out, a.Entity.Name)
		}
	}
	sort.Strings(out)
	return out
}

func TestAnalyzePaymentChangeReachesOperationAndWorkflow(t *testing.T) {
	m := model(t)
	imp := Analyze(m, []ChangedFile{{Path: "payment-service/fee.go", Status: "M"}})

	if got := len(imp.Changed); got != 1 || imp.Changed[0].Name != "payment-service/fee.go" {
		t.Fatalf("changed: %+v", imp.Changed)
	}
	// Both operations of payment-service are reachable, since fee.go belongs
	// to the whole service, not just one operation.
	if got := kindNames(imp, contextmodel.KindOperation); len(got) != 2 {
		t.Fatalf("expected both payment-service operations reached, got %v", got)
	}
	if got := kindNames(imp, contextmodel.KindWorkflow); len(got) != 1 || got[0] != "payment-create" {
		t.Fatalf("expected payment-create reached, got %v", got)
	}
	if len(imp.Plan.Workflows) != 1 || imp.Plan.Workflows[0].Name != "payment-create" {
		t.Fatalf("verification plan: %+v", imp.Plan)
	}
	if len(imp.Plan.Workflows[0].Reason) == 0 {
		t.Fatal("planned workflow must carry its relation chain")
	}
	if len(imp.Owners) != 1 || imp.Owners[0].Name != "@payments-team" {
		t.Fatalf("owners: %+v", imp.Owners)
	}

	// Every potential-impact entry must explain itself.
	for _, a := range imp.Potential {
		if len(a.Path) == 0 {
			t.Errorf("%s reached with no explanation path", a.Entity.ID)
		}
	}
}

func TestAnalyzeCheckoutServiceReachesPaymentDependency(t *testing.T) {
	m := model(t)
	imp := Analyze(m, []ChangedFile{{Path: "checkout-service/checkout.go", Status: "M"}})
	if got := kindNames(imp, contextmodel.KindService); len(got) == 0 {
		t.Fatal("expected checkout change to reach a dependent service")
	} else {
		found := false
		for _, n := range got {
			if n == "payment-service" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected payment-service among reached services, got %v", got)
		}
	}
}

// A change confined to docs must not produce any potential impact — the
// negative case the whole design exists to get right.
func TestAnalyzeUnrelatedChangeHasNoImpact(t *testing.T) {
	m := model(t)
	imp := Analyze(m, []ChangedFile{{Path: "docs-site/index.md", Status: "M"}})
	if len(imp.Potential) != 0 {
		t.Fatalf("expected no potential impact, got %+v", imp.Potential)
	}
	if len(imp.Plan.Workflows) != 0 {
		t.Fatalf("expected no verification plan, got %+v", imp.Plan)
	}
	// Ownership of the docs file is still reported — that is context, not impact.
	if len(imp.Owners) != 1 || imp.Owners[0].Name != "@platform-team" {
		t.Fatalf("owners: %+v", imp.Owners)
	}
}

func TestAnalyzeFileWithNoContextIsStillChanged(t *testing.T) {
	m := model(t)
	imp := Analyze(m, []ChangedFile{{Path: "does/not/exist.go", Status: "A"}})
	if len(imp.Changed) != 1 {
		t.Fatalf("changed: %+v", imp.Changed)
	}
	if len(imp.Notes) == 0 {
		t.Fatal("expected a note explaining the missing context")
	}
	if len(imp.Potential) != 0 {
		t.Fatalf("unknown file must reach nothing, got %+v", imp.Potential)
	}
}

func TestAnalyzeRespectsDepthBound(t *testing.T) {
	m := model(t)
	imp := AnalyzeAtDepth(m, []ChangedFile{{Path: "payment-service/fee.go", Status: "M"}}, 1)
	if got := kindNames(imp, contextmodel.KindWorkflow); len(got) != 0 {
		t.Fatalf("depth 1 must not reach the workflow, got %v", got)
	}
}
