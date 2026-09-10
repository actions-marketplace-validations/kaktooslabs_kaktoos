package engine_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
)

const schemaTestSpec = `
openapi: 3.0.3
info: {title: T, version: 1.0.0}
paths:
  /users/{id}:
    get:
      operationId: getUser
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

func loadSchemaTestOps(t *testing.T) openapi.OperationMap {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yml")
	if err := os.WriteFile(p, []byte(schemaTestSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	ops, err := openapi.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func runGetUserStep(t *testing.T, mode schema.Mode, responseBody string) engine.StepResult {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	defer server.Close()

	env := config.Environment{BaseURL: server.URL}
	ops := loadSchemaTestOps(t)
	s := scenario.Scenario{
		Name: "schema test",
		Steps: []scenario.Step{
			{Name: "get", Operation: "getUser", Request: &scenario.RequestSpec{Path: map[string]string{"id": "1"}}},
		},
	}

	ctx := engine.NewContext(env, ops)
	ctx.SchemaMode = mode
	result := engine.RunScenario(ctx, s, httpclient.NewClient().HTTPClient())
	return result.Steps[0]
}

func TestSchemaMode_OffRunsNoValidation(t *testing.T) {
	step := runGetUserStep(t, schema.Off, `{"id":"1"}`) // missing required "name"
	if step.Status != engine.StepPassed {
		t.Fatalf("off mode must not validate schema, got status %s error %q", step.Status, step.Error)
	}
	if step.SchemaViolations != nil {
		t.Fatalf("off mode must not populate violations, got %+v", step.SchemaViolations)
	}
}

func TestSchemaMode_WarnReportsButPasses(t *testing.T) {
	step := runGetUserStep(t, schema.Warn, `{"id":"1"}`) // missing required "name"
	if step.Status != engine.StepPassed {
		t.Fatalf("warn mode must not fail the step, got status %s error %q", step.Status, step.Error)
	}
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != schema.KindRequiredFieldMissing {
		t.Fatalf("expected one required_field_missing violation, got %+v", step.SchemaViolations)
	}
}

func TestSchemaMode_StrictFailsOnSameViolation(t *testing.T) {
	step := runGetUserStep(t, schema.Strict, `{"id":"1"}`) // missing required "name"
	if step.Status != engine.StepFailed {
		t.Fatalf("strict mode must fail the step, got status %s", step.Status)
	}
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != schema.KindRequiredFieldMissing {
		t.Fatalf("expected one required_field_missing violation, got %+v", step.SchemaViolations)
	}
}

// TestSchemaMode_WarnAndStrictReportIdenticalViolations is the contract that
// Validate is mode-independent: only pass/fail differs, never the findings.
func TestSchemaMode_WarnAndStrictReportIdenticalViolations(t *testing.T) {
	warnStep := runGetUserStep(t, schema.Warn, `{"id":"1"}`)
	strictStep := runGetUserStep(t, schema.Strict, `{"id":"1"}`)
	if len(warnStep.SchemaViolations) != len(strictStep.SchemaViolations) {
		t.Fatalf("violation counts differ: warn=%v strict=%v", warnStep.SchemaViolations, strictStep.SchemaViolations)
	}
	for i := range warnStep.SchemaViolations {
		if warnStep.SchemaViolations[i] != strictStep.SchemaViolations[i] {
			t.Fatalf("violations differ at %d: warn=%+v strict=%+v", i, warnStep.SchemaViolations[i], strictStep.SchemaViolations[i])
		}
	}
}

func TestSchemaMode_StrictPassesOnValidResponse(t *testing.T) {
	step := runGetUserStep(t, schema.Strict, `{"id":"1","name":"n"}`)
	if step.Status != engine.StepPassed {
		t.Fatalf("expected pass, got %s error %q", step.Status, step.Error)
	}
	if len(step.SchemaViolations) != 0 {
		t.Fatalf("expected no violations, got %+v", step.SchemaViolations)
	}
}
