package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/contextmodel"
)

const fixture = "testdata/monorepo"

func discover(t *testing.T, root string) *contextmodel.Model {
	t.Helper()
	m, notes, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover(%s): %v", root, err)
	}
	for _, n := range notes {
		t.Logf("note: %s: %s", n.Source, n.Message)
	}
	return m
}

func TestDiscoverDeclaredServices(t *testing.T) {
	m := discover(t, fixture)
	for _, name := range []string{"payment-service", "checkout-service", "reporting-service"} {
		if _, ok := m.Get(contextmodel.ServiceID(name)); !ok {
			t.Errorf("expected service %s", name)
		}
	}
}

func TestDiscoverOperationsFromOpenAPI(t *testing.T) {
	m := discover(t, fixture)
	ops := m.Related(contextmodel.ServiceID("payment-service"), contextmodel.RelExposes)
	got := entityNames(ops)
	want := []string{"GET /payments/{id}", "POST /payments"}
	if len(got) != len(want) {
		t.Fatalf("operations: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("operations: got %v, want %v", got, want)
		}
	}
	// Declared services must not be marked inferred.
	for _, r := range m.RelationsFrom(contextmodel.ServiceID("payment-service"), contextmodel.RelExposes) {
		if r.Inferred {
			t.Error("kaktoos.yaml-declared relation must not be inferred")
		}
		if r.Why == "" {
			t.Error("relation must carry a Why")
		}
	}
}

func TestDiscoverWorkflowLinksToOperation(t *testing.T) {
	m := discover(t, fixture)
	wf := contextmodel.WorkflowID("payment-create")
	if _, ok := m.Get(wf); !ok {
		t.Fatal("expected workflow payment-create")
	}
	var linked bool
	for _, r := range m.RelationsFrom(wf, contextmodel.RelVerifies) {
		if r.To == contextmodel.OperationID("POST", "/payments") {
			linked = true
			if r.Why == "" {
				t.Error("workflow -> operation relation needs a Why")
			}
		}
	}
	if !linked {
		t.Error("workflow payment-create should verify POST /payments")
	}
}

func TestDiscoverDependsOn(t *testing.T) {
	m := discover(t, fixture)
	deps := m.Related(contextmodel.ServiceID("checkout-service"), contextmodel.RelDependsOn)
	if got := entityNames(deps); len(got) != 1 || got[0] != "payment-service" {
		t.Fatalf("checkout depends_on: %v", got)
	}
}

func TestDiscoverFilesMapToServices(t *testing.T) {
	m := discover(t, fixture)
	rels := m.RelationsFrom(contextmodel.FileID("payment-service/fee.go"), contextmodel.RelContains)
	if len(rels) != 1 || rels[0].To != contextmodel.ServiceID("payment-service") {
		t.Fatalf("fee.go should belong to payment-service, got %+v", rels)
	}
	// A file outside every declared glob belongs to no service.
	if got := m.RelationsFrom(contextmodel.FileID("docs-site/index.md"), contextmodel.RelContains); len(got) != 0 {
		t.Fatalf("docs-site file must not be attached to a service, got %+v", got)
	}
}

func TestDiscoverOwnership(t *testing.T) {
	m := discover(t, fixture)
	owners := m.Related(contextmodel.FileID("payment-service/fee.go"), contextmodel.RelOwnedBy)
	if got := entityNames(owners); len(got) != 1 || got[0] != "@payments-team" {
		t.Fatalf("fee.go owners: %v", got)
	}
	// The catch-all rule still owns unrelated files — last match wins.
	docs := m.Related(contextmodel.FileID("docs-site/index.md"), contextmodel.RelOwnedBy)
	if got := entityNames(docs); len(got) != 1 || got[0] != "@platform-team" {
		t.Fatalf("docs owners: %v", got)
	}
}

// With no kaktoos.yaml, a directory holding an openapi.yml is a service and
// everything derived from it is flagged inferred.
func TestDiscoverConventionFallbackMarksInferred(t *testing.T) {
	root := t.TempDir()
	svcDir := filepath.Join(root, "payment-service")
	if err := os.MkdirAll(filepath.Join(svcDir, "scenarios"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(fixture, "payment-service/openapi.yml"), filepath.Join(svcDir, "openapi.yml"))
	copyFile(t, filepath.Join(fixture, "payment-service/scenarios/payment-create.yml"), filepath.Join(svcDir, "scenarios/payment-create.yml"))

	m := discover(t, root)
	rels := m.RelationsFrom(contextmodel.ServiceID("payment-service"), contextmodel.RelExposes)
	if len(rels) == 0 {
		t.Fatal("convention fallback should expose operations")
	}
	for _, r := range rels {
		if !r.Inferred {
			t.Errorf("convention-derived relation must be marked inferred: %+v", r)
		}
	}
}

func TestDiscoverNoConfigNoSpecsIsEmptyNotAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _, err := Discover(root)
	if err != nil {
		t.Fatalf("empty repo should not error: %v", err)
	}
	if got := len(m.Entities()); got != 0 {
		t.Fatalf("expected no entities, got %d", got)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func entityNames(es []contextmodel.Entity) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}
