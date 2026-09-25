package engine_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
	"github.com/kaktooslabs/kaktoos/internal/verification"
)

// ordersSpec is a minimal write/read-back/delete contract.
const ordersSpec = `
openapi: 3.0.3
info: {title: Orders, version: 1.0.0}
paths:
  /orders:
    post:
      operationId: createOrder
      responses:
        "200":
          description: created
          content:
            application/json:
              schema:
                type: object
                required: [id]
                properties:
                  id: {type: string}
  /orders/{id}:
    get:
      operationId: getOrder
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [id]
                properties:
                  id: {type: string}
    delete:
      operationId: deleteOrder
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
      responses:
        "204": {description: gone}
`

func loadOrdersOps(t *testing.T) openapi.OperationMap {
	t.Helper()
	p := filepath.Join(t.TempDir(), "openapi.yml")
	if err := os.WriteFile(p, []byte(ordersSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	ops, err := openapi.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func runOrders(t *testing.T, baseURL string, mode schema.Mode, steps ...scenario.Step) engine.ExecutionResult {
	t.Helper()
	env := config.Environment{BaseURL: baseURL, Headers: map[string]string{"Authorization": "Bearer top-secret"}}
	ctx := engine.NewContext(env, loadOrdersOps(t))
	ctx.SchemaMode = mode
	return engine.RunScenario(ctx, scenario.Scenario{Name: "orders", Steps: steps}, httpclient.NewClient().HTTPClient())
}

func status(n int) *int { return &n }

func requireCategory(t *testing.T, s engine.StepResult, want verification.Category) {
	t.Helper()
	if s.Status == engine.StepPassed {
		t.Fatalf("step %q unexpectedly passed", s.Name)
	}
	if s.FailureCategory != want {
		t.Fatalf("step %q: category %q, want %q (error: %s)", s.Name, s.FailureCategory, want, s.Error)
	}
}

func requireRedactedEvidence(t *testing.T, s engine.StepResult) {
	t.Helper()
	if s.Evidence == nil {
		t.Fatalf("step %q: no evidence captured", s.Name)
	}
	if got := s.Evidence.RequestHeaders["Authorization"]; got != "***" {
		t.Fatalf("step %q: Authorization not redacted in evidence: %q", s.Name, got)
	}
	if s.Evidence.Method == "" || s.Evidence.URL == "" {
		t.Fatalf("step %q: evidence missing method/url: %+v", s.Name, s.Evidence)
	}
}

// The headline Phase 3B case: POST says 200, nothing is stored, read-back 404s.
func TestVerification_StateNotPersisted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && r.URL.Path == "/orders":
			w.Write([]byte(`{"id":"o-1"}`)) // lies: persists nothing
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()

	// schema.Off: this case is about observable state, not contract shape — a
	// 404 read-back is an undeclared status in the fixture spec, which would
	// otherwise register as its own (also true) contract_mismatch and mask
	// the point of the test.
	res := runOrders(t, server.URL, schema.Off,
		scenario.Step{Name: "create", Operation: "createOrder", Extract: map[string]string{"order_id": "$.id"}, Assert: &scenario.AssertSpec{Status: status(200)}},
		scenario.Step{Name: "read back", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "{{order_id}}"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	if res.Steps[0].Status != engine.StepPassed {
		t.Fatalf("write step must pass on its own response: %s", res.Steps[0].Error)
	}
	requireCategory(t, res.Steps[1], verification.CategoryAssertionFailed)
	requireRedactedEvidence(t, res.Steps[1])
	if res.Steps[1].Evidence.StatusCode != 404 || res.Steps[1].Evidence.ResponseBody != `{"error":"not found"}` {
		t.Fatalf("evidence should carry the read-back response: %+v", res.Steps[1].Evidence)
	}
	if res.Passed {
		t.Fatal("scenario must fail when read-back does not observe the written state")
	}
}

func TestVerification_HTMLFallbackIsContractMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html>Sign in</html>`))
	}))
	defer server.Close()

	res := runOrders(t, server.URL, schema.Strict,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	s := res.Steps[0]
	requireCategory(t, s, verification.CategoryContractMismatch)
	if len(s.SchemaViolations) == 0 || s.SchemaViolations[0].Kind != schema.KindContentTypeMismatch {
		t.Fatalf("expected content_type_mismatch violation, got %+v", s.SchemaViolations)
	}
	requireRedactedEvidence(t, s)
}

func TestVerification_401IsAuthFailureNotAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	res := runOrders(t, server.URL, schema.Strict,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	requireCategory(t, res.Steps[0], verification.CategoryAuthFailure)
	requireRedactedEvidence(t, res.Steps[0])
}

func TestVerification_429IsRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(429)
	}))
	defer server.Close()

	res := runOrders(t, server.URL, schema.Off,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	requireCategory(t, res.Steps[0], verification.CategoryRateLimited)
	if res.Steps[0].Evidence.ResponseHeaders["Retry-After"] != "1" {
		t.Fatalf("evidence should keep non-sensitive response headers: %+v", res.Steps[0].Evidence.ResponseHeaders)
	}
}

func TestVerification_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	res := runOrders(t, server.URL, schema.Off,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	requireCategory(t, res.Steps[0], verification.CategoryServerError)
}

func TestVerification_TransportFailure(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close() // nothing listening any more

	res := runOrders(t, url, schema.Off,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}},
	)
	requireCategory(t, res.Steps[0], verification.CategoryTransportFailure)
	requireRedactedEvidence(t, res.Steps[0]) // the request was built; that is evidence too
	if res.Steps[0].Evidence.StatusCode != 0 {
		t.Fatalf("transport failure must not carry a status code: %+v", res.Steps[0].Evidence)
	}
}

func TestVerification_Timeout(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer func() { once.Do(func() { close(release) }); server.Close() }()

	res := runOrders(t, server.URL, schema.Off,
		scenario.Step{Name: "get", Operation: "getOrder", Timeout: "50ms", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}},
	)
	once.Do(func() { close(release) })
	s := res.Steps[0]
	if s.Status != engine.StepTimedOut {
		t.Fatalf("expected TIMED_OUT, got %s (%s)", s.Status, s.Error)
	}
	if s.FailureCategory != verification.CategoryTimeout {
		t.Fatalf("category %q, want timeout", s.FailureCategory)
	}
	if s.Evidence == nil || s.Evidence.RequestHeaders["Authorization"] != "***" {
		t.Fatalf("timeout should still carry redacted request evidence: %+v", s.Evidence)
	}
	if res.Status != engine.ExecutionTimedOut {
		t.Fatalf("execution status %s, want TIMED_OUT", res.Status)
	}
}

func TestVerification_UnknownOperationIsConfigError(t *testing.T) {
	res := runOrders(t, "http://127.0.0.1:1", schema.Off,
		scenario.Step{Name: "typo", Operation: "getOrdr"},
	)
	requireCategory(t, res.Steps[0], verification.CategoryConfigError)
	if res.Steps[0].Evidence != nil {
		t.Fatalf("no request was made, so no evidence expected: %+v", res.Steps[0].Evidence)
	}
}

func TestVerification_PassingStepHasNoCategoryOrEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1"}`))
	}))
	defer server.Close()
	res := runOrders(t, server.URL, schema.Strict,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(200)}},
	)
	s := res.Steps[0]
	if s.Status != engine.StepPassed || s.FailureCategory != "" || s.Evidence != nil {
		t.Fatalf("passing step must carry no category/evidence: status=%s cat=%q ev=%+v", s.Status, s.FailureCategory, s.Evidence)
	}
}

// Warn-mode violations are informational and must not be reported as the cause.
func TestVerification_WarnModeViolationDoesNotMaskAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"nope":1}`)) // missing required id
	}))
	defer server.Close()
	res := runOrders(t, server.URL, schema.Warn,
		scenario.Step{Name: "get", Operation: "getOrder", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}, Assert: &scenario.AssertSpec{Status: status(201)}},
	)
	requireCategory(t, res.Steps[0], verification.CategoryAssertionFailed)
}
