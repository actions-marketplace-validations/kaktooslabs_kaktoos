package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const testOpenAPI = `
openapi: 3.0.3
info: {title: Test API, version: 1.0.0}
paths:
  /users:
    post:
      operationId: createUser
      requestBody:
        required: true
        content:
          application/json:
            schema: {type: object, properties: {name: {type: string}}}
      responses:
        "201":
          description: created
          content:
            application/json:
              schema: {type: object, properties: {id: {type: string}, name: {type: string}}}
`

const testScenario = `
name: Create user
steps:
  - name: Create user
    operation: createUser
    request:
      body: |
        {"name": "{{customerName}}"}
    assert:
      status: 201
`

// newTestBackend returns an httptest server implementing POST /users.
func newTestBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "u1", "name": "test"})
	}))
}

// newTestServer wires a webhook Server backed by fixture workflow files and
// the given backend base URL and auth config.
func newTestServer(t *testing.T, baseURL string, auth *AuthConfig, mapping map[string]string) *Server {
	t.Helper()
	dir := t.TempDir()

	openapiPath := filepath.Join(dir, "openapi.yml")
	envPath := filepath.Join(dir, "env.yml")
	scenarioPath := filepath.Join(dir, "scenario.yml")

	if err := os.WriteFile(openapiPath, []byte(testOpenAPI), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scenarioPath, []byte(testScenario), 0o644); err != nil {
		t.Fatal(err)
	}
	envContent := fmt.Sprintf("base_url: %s\nvariables:\n  customerName: default\n", baseURL)
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Server: ServerConfig{Port: 0, Host: "127.0.0.1"},
		Routes: []Route{
			{
				Path:   "/hooks/{id}",
				Method: "POST",
				Workflow: WorkflowRef{
					OpenAPIPath:     openapiPath,
					EnvironmentPath: envPath,
					ScenarioPath:    scenarioPath,
				},
				Auth:            auth,
				VariableMapping: mapping,
			},
		},
	}
	return NewServer(cfg, 0, "127.0.0.1")
}

func TestServer_SuccessfulWorkflow(t *testing.T) {
	backend := newTestBackend(t)
	defer backend.Close()

	// webhookName does not collide with any env variable, so it survives seeding.
	s := newTestServer(t, backend.URL, nil, map[string]string{
		"webhookName": "body.name",
		"hookId":      "path.id",
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body WebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "PASSED" {
		t.Fatalf("expected PASSED, got %+v", body)
	}
	if body.ExecutionID == "" {
		t.Fatal("expected execution_id to be set")
	}
	if body.Duration == "" || body.StartTime == "" || body.EndTime == "" {
		t.Fatalf("expected timing fields set: %+v", body)
	}
	if body.ExtractedVariables["webhookName"] != "Alice" {
		t.Fatalf("expected body-extracted variable seeded, got %+v", body.ExtractedVariables)
	}
	if body.ExtractedVariables["hookId"] != "abc" {
		t.Fatalf("expected path-extracted variable seeded, got %+v", body.ExtractedVariables)
	}
}

func TestServer_EnvVariableOverridesWebhookVariable(t *testing.T) {
	backend := newTestBackend(t)
	defer backend.Close()

	// env.yml seeds customerName=default; webhook mapping also sets customerName.
	// Env must win (webhook vars have lower precedence).
	s := newTestServer(t, backend.URL, nil, map[string]string{"customerName": "body.name"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body WebhookResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.ExtractedVariables["customerName"] != "default" {
		t.Fatalf("expected env variable to override webhook variable, got %q", body.ExtractedVariables["customerName"])
	}
}

func TestServer_NoRouteMatch(t *testing.T) {
	s := newTestServer(t, "http://unused", nil, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/nope", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t, "http://unused", nil, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/hooks/abc")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServer_BodyTooLarge(t *testing.T) {
	s := newTestServer(t, "http://unused", nil, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	big := bytes.Repeat([]byte("a"), maxBodySize+1)
	resp, err := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestServer_AuthFailure(t *testing.T) {
	s := newTestServer(t, "http://unused", &AuthConfig{Type: "bearer_token", Token: "secret"}, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestServer_InvalidJSONBody(t *testing.T) {
	s := newTestServer(t, "http://unused", nil, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`not json`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServer_ExtractionFailureReturns400(t *testing.T) {
	s := newTestServer(t, "http://unused", nil, map[string]string{"x": "body.missing"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServer_InfrastructureFailureReturns500(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Routes: []Route{{
			Path:   "/hooks",
			Method: "POST",
			Workflow: WorkflowRef{
				OpenAPIPath:     filepath.Join(dir, "missing-openapi.yml"),
				EnvironmentPath: filepath.Join(dir, "missing-env.yml"),
				ScenarioPath:    filepath.Join(dir, "missing-scenario.yml"),
			},
		}},
	}
	s := NewServer(cfg, 0, "127.0.0.1")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/hooks", "application/json", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestServer_SharedStoreAndLimiterAcrossRequests(t *testing.T) {
	backend := newTestBackend(t)
	defer backend.Close()

	s := newTestServer(t, backend.URL, nil, nil)
	firstStore := s.idempotency
	firstLimiter := s.rateLimiter

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for i := 0; i < 3; i++ {
		resp, err := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	if s.idempotency != firstStore || s.rateLimiter != firstLimiter {
		t.Fatal("expected idempotency store and rate limiter to remain the same instance across requests")
	}
}

func TestServer_ConcurrentRequests(t *testing.T) {
	backend := newTestBackend(t)
	defer backend.Close()

	s := newTestServer(t, backend.URL, nil, nil)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Post(ts.URL+"/hooks/abc", "application/json", bytes.NewBufferString(`{}`))
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestMatchPath(t *testing.T) {
	params, ok := matchPath("/hooks/{id}/sub", "/hooks/42/sub")
	if !ok || params["id"] != "42" {
		t.Fatalf("expected match with id=42, got %+v ok=%v", params, ok)
	}

	if _, ok := matchPath("/hooks/{id}", "/hooks/42/extra"); ok {
		t.Fatal("expected segment count mismatch to fail")
	}

	if _, ok := matchPath("/hooks/fixed", "/hooks/other"); ok {
		t.Fatal("expected literal segment mismatch to fail")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	backend := newTestBackend(t)
	defer backend.Close()

	s := newTestServer(t, backend.URL, nil, nil)
	s.port = 0
	s.host = "127.0.0.1"

	done := make(chan error, 1)
	go func() { done <- s.Start() }()

	// Give the listener a moment to come up, then request shutdown.
	time.Sleep(100 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean shutdown, got %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("server did not shut down within the graceful shutdown deadline")
	}
}
