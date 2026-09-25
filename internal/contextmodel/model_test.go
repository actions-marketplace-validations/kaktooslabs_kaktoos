package contextmodel

import (
	"sort"
	"testing"
)

func names(es []Entity) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

func sample() *Model {
	m := New()
	m.Add(File("payment-service/fee.go"))
	m.Add(Service("payment-service"))
	m.Add(Service("checkout-service"))
	m.Add(Operation("POST", "/payments", "createPayment", "payment-service/openapi.yml"))
	m.Add(Workflow("payment-create", "payment-service/scenarios/create.yml"))
	m.Add(Owner("@payments-team"))

	m.Relate(FileID("payment-service/fee.go"), ServiceID("payment-service"), RelContains, "matches paths glob payment-service/**", false)
	m.Relate(ServiceID("payment-service"), OperationID("POST", "/payments"), RelExposes, "declared in payment-service/openapi.yml", false)
	m.Relate(WorkflowID("payment-create"), OperationID("POST", "/payments"), RelVerifies, "step operation createPayment", false)
	m.Relate(OperationID("POST", "/payments"), WorkflowID("payment-create"), RelVerifies, "verified by payment-create", false)
	m.Relate(ServiceID("checkout-service"), ServiceID("payment-service"), RelDependsOn, "declared depends_on", false)
	m.Relate(ServiceID("payment-service"), OwnerID("@payments-team"), RelOwnedBy, "CODEOWNERS payment-service/", false)
	return m
}

func TestAddAndGet(t *testing.T) {
	m := sample()
	e, ok := m.Get(ServiceID("payment-service"))
	if !ok || e.Kind != KindService || e.Name != "payment-service" {
		t.Fatalf("get service: %+v ok=%v", e, ok)
	}
	if _, ok := m.Get("service:nope"); ok {
		t.Fatal("unknown id should not resolve")
	}
}

func TestAddIsLastWriteWins(t *testing.T) {
	m := New()
	m.Add(Service("payment-service"))
	updated := Service("payment-service")
	updated.Attrs = map[string]string{"openapi": "spec.yml"}
	m.Add(updated)
	e, _ := m.Get(ServiceID("payment-service"))
	if e.Attrs["openapi"] != "spec.yml" {
		t.Fatalf("re-add should overwrite: %+v", e)
	}
	if got := len(m.Entities()); got != 1 {
		t.Fatalf("expected 1 entity, got %d", got)
	}
}

func TestRelatedFiltersByKind(t *testing.T) {
	m := sample()
	all := m.Related(ServiceID("payment-service"))
	if got := len(all); got != 2 {
		t.Fatalf("expected 2 one-hop neighbours, got %d (%v)", got, names(all))
	}
	exposed := m.Related(ServiceID("payment-service"), RelExposes)
	if got := names(exposed); len(got) != 1 || got[0] != "POST /payments" {
		t.Fatalf("exposes: %v", got)
	}
	owned := m.Related(ServiceID("payment-service"), RelOwnedBy)
	if got := names(owned); len(got) != 1 || got[0] != "@payments-team" {
		t.Fatalf("owned_by: %v", got)
	}
}

func TestRelationsFromCarryWhy(t *testing.T) {
	m := sample()
	rels := m.RelationsFrom(ServiceID("payment-service"), RelExposes)
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].Why == "" {
		t.Fatal("every relation must carry a Why")
	}
	if rels[0].Inferred {
		t.Fatal("declared relation must not be marked inferred")
	}
}

func TestReachableWalksAndRecordsPath(t *testing.T) {
	m := sample()
	paths := m.Reachable([]string{FileID("payment-service/fee.go")}, 4)

	want := []string{
		ServiceID("payment-service"),
		OperationID("POST", "/payments"),
		WorkflowID("payment-create"),
		OwnerID("@payments-team"),
	}
	for _, id := range want {
		if _, ok := paths[id]; !ok {
			t.Errorf("expected to reach %s, got %v", id, keys(paths))
		}
	}
	// The workflow is 3 hops away: file -> service -> operation -> workflow.
	if got := len(paths[WorkflowID("payment-create")]); got != 3 {
		t.Fatalf("workflow path length %d, want 3", got)
	}
	for id, path := range paths {
		if len(path) == 0 {
			t.Errorf("%s reached with an empty explanation path", id)
		}
	}
}

func TestReachableRespectsDepth(t *testing.T) {
	m := sample()
	paths := m.Reachable([]string{FileID("payment-service/fee.go")}, 1)
	if _, ok := paths[ServiceID("payment-service")]; !ok {
		t.Fatal("depth 1 should reach the service")
	}
	if _, ok := paths[OperationID("POST", "/payments")]; ok {
		t.Fatal("depth 1 must not reach the operation")
	}
}

// operation <-> workflow point at each other; the walk must still terminate.
func TestReachableIsCycleSafe(t *testing.T) {
	m := sample()
	paths := m.Reachable([]string{OperationID("POST", "/payments")}, 10)
	if _, ok := paths[OperationID("POST", "/payments")]; ok {
		t.Fatal("a seed must not appear in its own reachable set")
	}
	if got := len(paths[WorkflowID("payment-create")]); got != 1 {
		t.Fatalf("workflow should be 1 hop from the operation, got %d", got)
	}
}

func TestReachableExcludesSeeds(t *testing.T) {
	m := sample()
	seed := ServiceID("payment-service")
	if _, ok := m.Reachable([]string{seed}, 3)[seed]; ok {
		t.Fatal("seed must be excluded from its own reachable set")
	}
}

func keys(m map[string][]Relation) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
