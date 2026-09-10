package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/openapi"
)

const specYAML = `
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
                  age: {type: integer}
        "204":
          description: no content
`

func loadGetUser(t testing.TB) openapi.Operation {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yml")
	if err := os.WriteFile(p, []byte(specYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	ops, err := openapi.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	op, _, _, found := openapi.FindOperation(ops, "getUser")
	if !found {
		t.Fatal("operation not found")
	}
	return *op
}

func TestValidate_StatusUndeclared(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 404, "application/json", []byte(`{}`))
	if len(r.Violations) != 1 || r.Violations[0].Kind != KindStatusUndeclared {
		t.Fatalf("expected status_undeclared, got %+v", r.Violations)
	}
}

func TestValidate_ContentTypeMismatch(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "text/plain", []byte(`hello`))
	if len(r.Violations) != 1 || r.Violations[0].Kind != KindContentTypeMismatch {
		t.Fatalf("expected content_type_mismatch, got %+v", r.Violations)
	}
}

func TestValidate_ContentTypeWithCharsetMatches(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "application/json; charset=utf-8", []byte(`{"id":"1","name":"n"}`))
	if len(r.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", r.Violations)
	}
}

func TestValidate_RequiredFieldMissing(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "application/json", []byte(`{"id":"1"}`))
	if len(r.Violations) != 1 || r.Violations[0].Kind != KindRequiredFieldMissing {
		t.Fatalf("expected required_field_missing, got %+v", r.Violations)
	}
}

func TestValidate_SchemaMismatch(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "application/json", []byte(`{"id":"1","name":"n","age":"not-a-number"}`))
	if len(r.Violations) != 1 || r.Violations[0].Kind != KindSchemaMismatch {
		t.Fatalf("expected schema_mismatch, got %+v", r.Violations)
	}
}

func TestValidate_UndeclaredFieldIsInformationalOnly(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "application/json", []byte(`{"id":"1","name":"n","extra":"x"}`))
	if len(r.Violations) != 0 {
		t.Fatalf("undeclared field must not be a violation, got %+v", r.Violations)
	}
	if len(r.UndeclaredFields) != 1 || r.UndeclaredFields[0] != "$.extra" {
		t.Fatalf("expected undeclared field $.extra, got %+v", r.UndeclaredFields)
	}
}

func TestValidate_NoContentStatusSkipsBodyChecks(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 204, "", nil)
	if len(r.Violations) != 0 || len(r.UndeclaredFields) != 0 {
		t.Fatalf("expected no checks for empty-content status, got %+v", r)
	}
}

func TestValidate_ValidResponseHasNoViolations(t *testing.T) {
	op := loadGetUser(t)
	r := Validate(op, 200, "application/json", []byte(`{"id":"1","name":"n"}`))
	if len(r.Violations) != 0 {
		t.Fatalf("expected no violations, got %+v", r.Violations)
	}
}

const openSpecYAML = `
openapi: 3.0.3
info: {title: T, version: 1.0.0}
paths:
  /things:
    get:
      operationId: listThings
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                additionalProperties: true
                properties:
                  id: {type: string}
`

func TestValidate_AdditionalPropertiesTrueSuppressesUndeclared(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "openapi.yml")
	if err := os.WriteFile(p, []byte(openSpecYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	ops, err := openapi.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	op, _, _, _ := openapi.FindOperation(ops, "listThings")
	r := Validate(*op, 200, "application/json", []byte(`{"id":"1","whatever":true}`))
	if len(r.UndeclaredFields) != 0 {
		t.Fatalf("additionalProperties:true must suppress undeclared detection, got %+v", r.UndeclaredFields)
	}
}

// Mode itself never changes what Validate reports: warn and strict callers
// see byte-for-byte identical violations. Only the caller's pass/fail
// decision (exercised at the engine level) differs.
func TestValidate_IsModeIndependent(t *testing.T) {
	op := loadGetUser(t)
	body := []byte(`{"id":"1"}`)
	a := Validate(op, 200, "application/json", body)
	b := Validate(op, 200, "application/json", body)
	if len(a.Violations) != len(b.Violations) || a.Violations[0] != b.Violations[0] {
		t.Fatalf("Validate must be deterministic and mode-independent: %+v vs %+v", a, b)
	}
}
