package engine_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
)

// brokenStore returns 200 on create, 404 on read-back, and counts deletes.
func brokenStore(deletes *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST":
			w.Write([]byte(`{"id":"o-1"}`))
		case r.Method == "DELETE":
			atomic.AddInt32(deletes, 1)
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
}

func cleanupSteps(alwaysRun bool) []scenario.Step {
	return []scenario.Step{
		{Name: "create", Operation: "createOrder", Extract: map[string]string{"order_id": "$.id"}},
		{Name: "read back", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "{{order_id}}"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
		{Name: "cleanup", Operation: "deleteOrder", AlwaysRun: alwaysRun, Request: &scenario.RequestSpec{Path: map[string]string{"id": "{{order_id}}"}}, Assert: &scenario.AssertSpec{Status: status(204)}},
	}
}

func TestAlwaysRun_CleanupRunsAfterFailure(t *testing.T) {
	var deletes int32
	server := brokenStore(&deletes)
	defer server.Close()

	res := runOrders(t, server.URL, schema.Off, cleanupSteps(true)...)

	if res.Steps[1].Status != engine.StepFailed {
		t.Fatalf("read-back should fail, got %s", res.Steps[1].Status)
	}
	if res.Steps[2].Status != engine.StepPassed {
		t.Fatalf("cleanup should run and pass, got %s (%s)", res.Steps[2].Status, res.Steps[2].Error)
	}
	if atomic.LoadInt32(&deletes) != 1 {
		t.Fatalf("expected exactly one DELETE, got %d", deletes)
	}
	if res.Passed || res.Status != engine.ExecutionFailed {
		t.Fatalf("a passing cleanup must not rescue the scenario: passed=%v status=%s", res.Passed, res.Status)
	}
}

func TestAlwaysRun_DefaultStillSkips(t *testing.T) {
	var deletes int32
	server := brokenStore(&deletes)
	defer server.Close()

	res := runOrders(t, server.URL, schema.Off, cleanupSteps(false)...)

	if res.Steps[2].Status != engine.StepSkipped {
		t.Fatalf("without always_run the cleanup must be SKIPPED, got %s", res.Steps[2].Status)
	}
	if atomic.LoadInt32(&deletes) != 0 {
		t.Fatalf("expected no DELETE, got %d", deletes)
	}
}
