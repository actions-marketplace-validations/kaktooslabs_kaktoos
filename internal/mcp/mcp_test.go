package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSpec = `
openapi: 3.0.3
info: {title: T, version: 1.0.0}
paths:
  /users/{id}:
    get:
      operationId: getUser
      tags: [users]
      parameters:
        - name: id
          in: path
          required: true
          schema: {type: string}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                required: [id, name]
                properties:
                  id: {type: string}
                  name: {type: string}
`

const testScenario = `
name: get user
steps:
  - name: get
    operation: getUser
    request:
      path:
        id: "1"
`

func writeSpec(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "openapi.yml")
	if err := os.WriteFile(p, []byte(testSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// --- list_operations ---

func TestListOperations(t *testing.T) {
	res, out, err := ListOperations(context.Background(), nil, ListOperationsInput{OpenAPIPath: writeSpec(t)})
	if err != nil || res != nil {
		t.Fatalf("unexpected failure: res=%v err=%v", res, err)
	}
	if len(out.Operations) != 1 {
		t.Fatalf("expected one operation, got %+v", out.Operations)
	}
	op := out.Operations[0]
	if op.OperationID != "getUser" || op.Method != "GET" || op.Path != "/users/{id}" {
		t.Fatalf("unexpected operation summary: %+v", op)
	}
	if len(op.Parameters) != 1 || op.Parameters[0].Name != "id" || !op.Parameters[0].Required {
		t.Fatalf("expected the required path parameter, got %+v", op.Parameters)
	}
	if len(op.Responses) != 1 || op.Responses[0] != "200" {
		t.Fatalf("expected declared response 200, got %+v", op.Responses)
	}
}

func TestListOperations_FilterExcludes(t *testing.T) {
	_, out, _ := ListOperations(context.Background(), nil, ListOperationsInput{OpenAPIPath: writeSpec(t), Filter: "nothing-matches"})
	if len(out.Operations) != 0 {
		t.Fatalf("expected no matches, got %+v", out.Operations)
	}
}

func TestListOperations_MissingSpecIsStructuredError(t *testing.T) {
	res, _, err := ListOperations(context.Background(), nil, ListOperationsInput{OpenAPIPath: "/nonexistent/openapi.yml"})
	if err != nil {
		t.Fatalf("a missing file must be a tool error, not a Go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result, got %+v", res)
	}
}

// --- validate_scenario ---

func TestValidateScenario_Valid(t *testing.T) {
	_, out, _ := ValidateScenario(context.Background(), nil, ValidateScenarioInput{ScenarioYAML: testScenario, OpenAPIPath: writeSpec(t)})
	if !out.Valid || len(out.Errors) != 0 {
		t.Fatalf("expected valid, got %+v", out)
	}
}

func TestValidateScenario_UnknownOperation(t *testing.T) {
	bad := strings.Replace(testScenario, "getUser", "getUsr", 1)
	_, out, _ := ValidateScenario(context.Background(), nil, ValidateScenarioInput{ScenarioYAML: bad, OpenAPIPath: writeSpec(t)})
	if out.Valid || len(out.Errors) != 1 || !strings.Contains(out.Errors[0], "getUsr") {
		t.Fatalf("expected the typo'd operation reported, got %+v", out)
	}
}

func TestValidateScenario_MalformedYAML(t *testing.T) {
	_, out, err := ValidateScenario(context.Background(), nil, ValidateScenarioInput{ScenarioYAML: "steps: [[[ nope"})
	if err != nil {
		t.Fatalf("malformed YAML must not be a Go error: %v", err)
	}
	if out.Valid || len(out.Errors) == 0 {
		t.Fatalf("expected invalid with errors, got %+v", out)
	}
}

// --- run_workflow ---

func runAgainst(t *testing.T, handler http.HandlerFunc, scenarioYAML string) RunWorkflowOutput {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	_, out, err := RunWorkflow(context.Background(), nil, RunWorkflowInput{
		ScenarioYAML: scenarioYAML,
		OpenAPIPath:  writeSpec(t),
		Environment:  fmt.Sprintf("base_url: %s\n", server.URL),
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	return out
}

func TestRunWorkflow_Passes(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","name":"n"}`))
	}, testScenario)
	if !out.Passed {
		t.Fatalf("expected pass, got %+v", out.Steps)
	}
}

// The four failure categories below are the point of this tool: an agent must
// be able to tell them apart from the structured output alone.

func TestRunWorkflow_DistinguishesRequiredFieldMissing(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1"}`))
	}, testScenario)
	step := out.Steps[0]
	if out.Passed || len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != "required_field_missing" {
		t.Fatalf("expected required_field_missing, got %+v", step)
	}
	if step.SchemaViolations[0].Path != "$.name" {
		t.Fatalf("expected the missing field pinpointed at $.name, got %q", step.SchemaViolations[0].Path)
	}
}

func TestRunWorkflow_DistinguishesSchemaMismatch(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1,"name":"n"}`)) // id declared as string
	}, testScenario)
	step := out.Steps[0]
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != "schema_mismatch" {
		t.Fatalf("expected schema_mismatch, got %+v", step)
	}
}

func TestRunWorkflow_DistinguishesContentTypeMismatch(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(`hello`))
	}, testScenario)
	step := out.Steps[0]
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != "content_type_mismatch" {
		t.Fatalf("expected content_type_mismatch, got %+v", step)
	}
}

func TestRunWorkflow_DistinguishesStatusUndeclared(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"nope"}`))
	}, testScenario)
	step := out.Steps[0]
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != "status_undeclared" {
		t.Fatalf("expected status_undeclared, got %+v", step)
	}
	if step.StatusCode != 404 {
		t.Fatalf("expected the real status code reported, got %d", step.StatusCode)
	}
}

func TestRunWorkflow_DistinguishesAssertionFailure(t *testing.T) {
	scn := testScenario + "    assert:\n      status: 201\n"
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","name":"n"}`))
	}, scn)
	step := out.Steps[0]
	if len(step.AssertionFailures) != 1 {
		t.Fatalf("expected one assertion failure, got %+v", step)
	}
	if len(step.SchemaViolations) != 0 {
		t.Fatalf("a valid response must produce no schema violations, got %+v", step.SchemaViolations)
	}
}

func TestRunWorkflow_DistinguishesTransportFailure(t *testing.T) {
	// Point at a closed port: the step must report an Error and no StatusCode.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	_, out, err := RunWorkflow(context.Background(), nil, RunWorkflowInput{
		ScenarioYAML: testScenario,
		OpenAPIPath:  writeSpec(t),
		Environment:  fmt.Sprintf("base_url: %s\n", url),
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	step := out.Steps[0]
	if step.Error == "" || step.StatusCode != 0 {
		t.Fatalf("expected a transport error with no status code, got %+v", step)
	}
	if len(step.SchemaViolations) != 0 || len(step.AssertionFailures) != 0 {
		t.Fatalf("a transport failure must not masquerade as a contract failure: %+v", step)
	}
}

func TestRunWorkflow_UndeclaredFieldIsInformationalOnly(t *testing.T) {
	out := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","name":"n","extra":true}`))
	}, testScenario)
	step := out.Steps[0]
	if !out.Passed {
		t.Fatalf("undeclared fields must never fail a workflow, got %+v", step)
	}
	if len(step.UndeclaredFields) != 1 || step.UndeclaredFields[0] != "$.extra" {
		t.Fatalf("expected $.extra reported informationally, got %+v", step.UndeclaredFields)
	}
}

func TestRunWorkflow_InvalidSchemaModeIsStructuredError(t *testing.T) {
	res, _, err := RunWorkflow(context.Background(), nil, RunWorkflowInput{
		ScenarioYAML: testScenario,
		OpenAPIPath:  writeSpec(t),
		Environment:  "base_url: http://localhost:1\n",
		SchemaMode:   "bogus",
	})
	if err != nil {
		t.Fatalf("expected a tool error, not a Go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result, got %+v", res)
	}
}

func TestRunWorkflow_MissingEnvironmentIsStructuredError(t *testing.T) {
	res, _, err := RunWorkflow(context.Background(), nil, RunWorkflowInput{
		ScenarioYAML: testScenario,
		OpenAPIPath:  writeSpec(t),
	})
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("expected IsError result, got res=%+v err=%v", res, err)
	}
}
