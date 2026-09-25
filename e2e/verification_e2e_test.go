//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

const verifSpec = `
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
              schema: {type: object, required: [id], properties: {id: {type: string}}}
  /orders/{id}:
    get:
      operationId: getOrder
      parameters: [{name: id, in: path, required: true, schema: {type: string}}]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, required: [id], properties: {id: {type: string}}}
    delete:
      operationId: deleteOrder
      parameters: [{name: id, in: path, required: true, schema: {type: string}}]
      responses:
        "204": {description: gone}
`

const verifScenario = `
name: order-state
steps:
  - name: create
    operation: createOrder
    extract: {order_id: $.id}
    assert: {status: 200}
  - name: read back
    operation: getOrder
    request: {path: {id: "{{order_id}}"}}
    assert: {status: 200}
  - name: cleanup
    operation: deleteOrder
    always_run: true
    request: {path: {id: "{{order_id}}"}}
    assert: {status: 204}
`

type verifJSON struct {
	Executions []struct {
		Steps []struct {
			Name            string `json:"name"`
			Status          string `json:"status"`
			FailureCategory string `json:"failure_category"`
			Evidence        *struct {
				StatusCode     int               `json:"status_code"`
				RequestHeaders map[string]string `json:"request_headers"`
				ResponseBody   string            `json:"response_body"`
			} `json:"evidence"`
		} `json:"steps"`
	} `json:"executions"`
}

// POST says 200 and persists nothing; the read-back is what catches it.
func TestE2E_StateNotPersisted(t *testing.T) {
	var deletes int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case "POST":
			w.Write([]byte(`{"id":"o-1"}`))
		case "DELETE":
			atomic.AddInt32(&deletes, 1)
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer server.Close()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "kaktoos-verif-test")
	build := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	defer os.Remove(binary)

	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	spec := write("openapi.yml", verifSpec)
	env := write("env.yml", "base_url: "+server.URL+"\nheaders:\n  Authorization: Bearer top-secret\n")
	scn := write("scenario.yml", verifScenario)

	out, err := exec.Command(binary, "run", "--openapi", spec, "--env", env, "--scenario", scn,
		"--schema-mode", "off", "--output-format", "json").Output()
	if err == nil {
		t.Fatalf("expected non-zero exit, got success:\n%s", out)
	}

	var got verifJSON
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	steps := got.Executions[0].Steps
	if steps[0].Status != "PASSED" {
		t.Fatalf("create should pass on its own response: %+v", steps[0])
	}
	rb := steps[1]
	if rb.Status != "FAILED" || rb.FailureCategory != "assertion_failed" {
		t.Fatalf("read back: status=%s category=%s", rb.Status, rb.FailureCategory)
	}
	if rb.Evidence == nil || rb.Evidence.StatusCode != 404 || rb.Evidence.RequestHeaders["Authorization"] != "***" {
		t.Fatalf("read back evidence wrong: %+v", rb.Evidence)
	}
	if steps[2].Status != "PASSED" || atomic.LoadInt32(&deletes) != 1 {
		t.Fatalf("cleanup should run once via always_run: status=%s deletes=%d", steps[2].Status, deletes)
	}
}

func TestE2E_AuthFailureCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()

	root, _ := filepath.Abs("..")
	binary := filepath.Join(root, "kaktoos-verif-test2")
	build := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	defer os.Remove(binary)

	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.yml")
	env := filepath.Join(dir, "env.yml")
	scn := filepath.Join(dir, "scenario.yml")
	os.WriteFile(spec, []byte(verifSpec), 0o644)
	os.WriteFile(env, []byte("base_url: "+server.URL+"\n"), 0o644)
	os.WriteFile(scn, []byte("name: auth\nsteps:\n  - name: get\n    operation: getOrder\n    request: {path: {id: \"1\"}}\n    assert: {status: 200}\n"), 0o644)

	out, err := exec.Command(binary, "run", "--openapi", spec, "--env", env, "--scenario", scn,
		"--schema-mode", "off", "--output-format", "json").Output()
	if err == nil {
		t.Fatalf("expected failure:\n%s", out)
	}
	var got verifJSON
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	if c := got.Executions[0].Steps[0].FailureCategory; c != "auth_failure" {
		t.Fatalf("category %q, want auth_failure (a 401 is not a bad assertion)", c)
	}
}
