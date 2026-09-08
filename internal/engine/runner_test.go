package engine_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

// TestRunScenario_Success simulates a successful scenario execution.
func TestRunScenario_Success(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"id": "123", "name": "TestUser", "status": "active"}`)
	}))
	defer server.Close()

	// Setup environment
	env := config.Environment{
		BaseURL:   server.URL,
		Headers:   map[string]string{"X-Test": "value"},
		Variables: map[string]string{"user_id": "123"},
	}

	// Setup OpenAPI operations map
	opMap := openapi.OperationMap{
		"/users/{id}": openapi.OperationMapInner{
			"GET": openapi.Operation{
				Name:        "getUserById",
				Description: "Get user by ID",
			},
		},
	}

	// Setup scenario
	statusCode := 200
	s := scenario.Scenario{
		Name: "Get User Success",
		Steps: []scenario.Step{
			{
				Name:      "fetch_user",
				Operation: "getUserById",
				Request: &scenario.RequestSpec{
					Path: map[string]string{"id": "{{user_id}}"},
				},
				Extract: map[string]string{
					"username": "$.name",
				},
				Assert: &scenario.AssertSpec{
					Status: &statusCode,
					Body: []scenario.BodyAssert{
						{Path: "$.status", Equals: "active"},
					},
				},
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if !result.Passed {
		t.Errorf("Expected scenario to pass, got failed. Steps: %+v", result.Steps)
	}
	if len(result.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != engine.StepPassed {
		t.Errorf("Expected step to pass, got %s. Error: %s", result.Steps[0].Status, result.Steps[0].Error)
	}

	// Verify variable was extracted
	extractedName, ok := ctx.VariableStore.Get("username")
	if !ok {
		t.Error("Expected username to be extracted")
	}
	if extractedName != "TestUser" {
		t.Errorf("Expected username to be 'TestUser', got '%s'", extractedName)
	}
}

// TestRunScenario_FailureAndSkip tests that after a step fails, subsequent steps are skipped.
func TestRunScenario_FailureAndSkip(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "inactive"}`)
	}))
	defer server.Close()

	// Setup environment
	env := config.Environment{
		BaseURL:   server.URL,
		Headers:   map[string]string{},
		Variables: map[string]string{},
	}

	// Setup OpenAPI operations map
	opMap := openapi.OperationMap{
		"/users": openapi.OperationMapInner{
			"GET": openapi.Operation{
				Name: "getUsers",
			},
		},
	}

	// Setup scenario with 3 steps: first fails, others should be skipped
	statusCode := 200
	s := scenario.Scenario{
		Name: "Failure Test",
		Steps: []scenario.Step{
			{
				Name:      "failing_step",
				Operation: "getUsers",
				Assert: &scenario.AssertSpec{
					Status: &statusCode,
					Body: []scenario.BodyAssert{
						{Path: "$.status", Equals: "active"}, // This will fail
					},
				},
			},
			{
				Name:      "skipped_step_1",
				Operation: "getUsers",
			},
			{
				Name:      "skipped_step_2",
				Operation: "getUsers",
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if result.Passed {
		t.Error("Expected scenario to fail")
	}
	if len(result.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(result.Steps))
	}

	// First step should fail
	if result.Steps[0].Status != engine.StepFailed {
		t.Errorf("Expected first step to fail, got %s", result.Steps[0].Status)
	}

	// Subsequent steps should be skipped
	if result.Steps[1].Status != engine.StepSkipped {
		t.Errorf("Expected second step to be skipped, got %s", result.Steps[1].Status)
	}
	if result.Steps[2].Status != engine.StepSkipped {
		t.Errorf("Expected third step to be skipped, got %s", result.Steps[2].Status)
	}
}

// TestRunScenario_MissingOperation tests that a missing operation fails gracefully.
func TestRunScenario_MissingOperation(t *testing.T) {
	// Setup environment
	env := config.Environment{
		BaseURL:   "http://localhost",
		Headers:   map[string]string{},
		Variables: map[string]string{},
	}

	// Empty operation map
	opMap := openapi.OperationMap{}

	// Scenario referencing non-existent operation
	s := scenario.Scenario{
		Name: "Missing Operation Test",
		Steps: []scenario.Step{
			{
				Name:      "missing_op_step",
				Operation: "nonExistentOp",
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if result.Passed {
		t.Error("Expected scenario to fail due to missing operation")
	}
	if result.Steps[0].Status != engine.StepFailed {
		t.Errorf("Expected step to fail, got %s", result.Steps[0].Status)
	}
	if result.Steps[0].Error == "" {
		t.Error("Expected error message for missing operation")
	}
}

// TestRunScenario_VariableSubstitution tests variable substitution in requests.
func TestRunScenario_VariableSubstitution(t *testing.T) {
	// Setup mock HTTP server that verifies the substituted path
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/42" {
			t.Errorf("Expected path /users/42, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("status") != "active" {
			t.Errorf("Expected query status=active, got %s", r.URL.Query().Get("status"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"result": "ok"}`)
	}))
	defer server.Close()

	// Setup environment with variables
	env := config.Environment{
		BaseURL: server.URL,
		Variables: map[string]string{
			"userId":     "42",
			"userStatus": "active",
		},
	}

	// Setup OpenAPI operations map
	opMap := openapi.OperationMap{
		"/users/{id}": openapi.OperationMapInner{
			"GET": openapi.Operation{
				Name: "getUser",
			},
		},
	}

	// Setup scenario with variable substitution
	s := scenario.Scenario{
		Name: "Variable Substitution Test",
		Steps: []scenario.Step{
			{
				Name:      "fetch_with_vars",
				Operation: "getUser",
				Request: &scenario.RequestSpec{
					Path: map[string]string{
						"id": "{{userId}}",
					},
					Query: map[string]string{
						"status": "{{userStatus}}",
					},
				},
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if !result.Passed {
		t.Errorf("Expected scenario to pass, got failed: %+v", result.Steps)
	}
}

// TestRunScenario_AssertionVariableSubstitution tests that variables are substituted in assertion expected values.
func TestRunScenario_AssertionVariableSubstitution(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users" && r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id": "user-123", "name": "Alice"}`)
		} else if r.URL.Path == "/users/user-123" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id": "user-123", "name": "Alice", "status": "active"}`)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Setup environment
	env := config.Environment{
		BaseURL:   server.URL,
		Variables: map[string]string{},
	}

	// Setup OpenAPI operations
	opMap := openapi.OperationMap{
		"/users": openapi.OperationMapInner{
			"POST": openapi.Operation{Name: "createUser"},
		},
		"/users/{id}": openapi.OperationMapInner{
			"GET": openapi.Operation{Name: "getUser"},
		},
	}

	// Setup scenario: create user, extract ID, get user with extracted ID in assertion
	status201 := 201
	status200 := 200
	s := scenario.Scenario{
		Name: "Variable Substitution in Assertion",
		Steps: []scenario.Step{
			{
				Name:      "create_user",
				Operation: "createUser",
				Extract: map[string]string{
					"userId": "$.id",
				},
				Assert: &scenario.AssertSpec{
					Status: &status201,
					Body: []scenario.BodyAssert{
						{Path: "$.name", Equals: "Alice"},
					},
				},
			},
			{
				Name:      "get_user",
				Operation: "getUser",
				Request: &scenario.RequestSpec{
					Path: map[string]string{"id": "{{userId}}"},
				},
				Assert: &scenario.AssertSpec{
					Status: &status200,
					Body: []scenario.BodyAssert{
						{Path: "$.id", Equals: "{{userId}}"}, // Variable substitution in assertion
						{Path: "$.name", Equals: "Alice"},
					},
				},
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if !result.Passed {
		t.Errorf("Expected scenario to pass, got failed")
		for i, step := range result.Steps {
			t.Logf("Step %d (%s): %s", i, step.Name, step.Status)
			if step.Error != "" {
				t.Logf("  Error: %s", step.Error)
			}
			for _, assertion := range step.Assertions {
				if !assertion.Passed {
					t.Logf("  Assertion %s failed: expected %v, got %v", assertion.Type, assertion.Expected, assertion.Actual)
				}
			}
		}
	}

	// Verify both steps passed
	if len(result.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != engine.StepPassed {
		t.Errorf("Expected first step to pass, got %s: %s", result.Steps[0].Status, result.Steps[0].Error)
	}
	if result.Steps[1].Status != engine.StepPassed {
		t.Errorf("Expected second step to pass, got %s: %s", result.Steps[1].Status, result.Steps[1].Error)
	}

	// Verify assertion on second step
	if len(result.Steps[1].Assertions) < 2 {
		t.Fatalf("Expected at least 2 assertions on second step, got %d", len(result.Steps[1].Assertions))
	}

	// Find the ID assertion
	var idAssertionPassed bool
	for _, a := range result.Steps[1].Assertions {
		if a.Path == "$.id" {
			idAssertionPassed = a.Passed
			if !a.Passed {
				t.Errorf("ID assertion failed: expected %v, got %v", a.Expected, a.Actual)
			}
			// Verify the variable was substituted (should be "user-123", not "{{userId}}")
			if a.Expected == "{{userId}}" {
				t.Error("Variable was not substituted in assertion expected value")
			}
			if a.Expected != "user-123" {
				t.Errorf("Expected assertion value to be 'user-123' after substitution, got %v", a.Expected)
			}
		}
	}
	if !idAssertionPassed {
		t.Error("ID assertion did not pass")
	}
}

// TestRunScenario_AssertionLiteralValues tests that literal (non-variable) assertion values still work.
func TestRunScenario_AssertionLiteralValues(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"count": 42, "enabled": true, "message": "success"}`)
	}))
	defer server.Close()

	// Setup environment
	env := config.Environment{
		BaseURL:   server.URL,
		Variables: map[string]string{},
	}

	// Setup OpenAPI operations
	opMap := openapi.OperationMap{
		"/status": openapi.OperationMapInner{
			"GET": openapi.Operation{Name: "getStatus"},
		},
	}

	// Setup scenario with literal values (string, number, bool)
	status200 := 200
	s := scenario.Scenario{
		Name: "Literal Assertion Values",
		Steps: []scenario.Step{
			{
				Name:      "check_status",
				Operation: "getStatus",
				Assert: &scenario.AssertSpec{
					Status: &status200,
					Body: []scenario.BodyAssert{
						{Path: "$.count", Equals: float64(42)}, // number
						{Path: "$.enabled", Equals: true},      // bool
						{Path: "$.message", Equals: "success"}, // string literal
					},
				},
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify
	if !result.Passed {
		t.Errorf("Expected scenario to pass with literal values")
		for _, step := range result.Steps {
			if step.Error != "" {
				t.Logf("Error: %s", step.Error)
			}
			for _, assertion := range step.Assertions {
				if !assertion.Passed {
					t.Logf("Assertion failed: %s - expected %v (%T), got %v (%T)",
						assertion.Path, assertion.Expected, assertion.Expected, assertion.Actual, assertion.Actual)
				}
			}
		}
	}
}

// TestRunScenario_AssertionMissingVariable tests that missing variables in assertions fail appropriately.
func TestRunScenario_AssertionMissingVariable(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"id": "123"}`)
	}))
	defer server.Close()

	// Setup environment with NO variables
	env := config.Environment{
		BaseURL:   server.URL,
		Variables: map[string]string{},
	}

	// Setup OpenAPI operations
	opMap := openapi.OperationMap{
		"/resource": openapi.OperationMapInner{
			"GET": openapi.Operation{Name: "getResource"},
		},
	}

	// Setup scenario with assertion referencing non-existent variable
	status200 := 200
	s := scenario.Scenario{
		Name: "Missing Variable in Assertion",
		Steps: []scenario.Step{
			{
				Name:      "check_resource",
				Operation: "getResource",
				Assert: &scenario.AssertSpec{
					Status: &status200,
					Body: []scenario.BodyAssert{
						{Path: "$.id", Equals: "{{nonExistentVar}}"}, // Variable not defined
					},
				},
			},
		},
	}

	// Execute
	ctx := engine.NewContext(env, opMap)
	result := engine.RunScenario(ctx, s, http.DefaultClient)

	// Verify scenario failed
	if result.Passed {
		t.Error("Expected scenario to fail due to missing variable")
	}

	// Verify step failed with appropriate error
	if len(result.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != engine.StepFailed {
		t.Errorf("Expected step to fail, got %s", result.Steps[0].Status)
	}
	if result.Steps[0].Error == "" {
		t.Error("Expected error message for missing variable")
	}
	// Error should mention the variable name
	if result.Steps[0].Error == "" {
		t.Error("Expected error message to mention variable")
	}
}
