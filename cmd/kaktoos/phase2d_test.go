package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

// --- 2D-2: tag filtering ---

func TestFilterByTags(t *testing.T) {
	smoke := &scenario.Scenario{Name: "smoke", Tags: []string{"smoke", "fast"}}
	slow := &scenario.Scenario{Name: "slow", Tags: []string{"regression"}}
	untagged := &scenario.Scenario{Name: "untagged"}
	all := []*scenario.Scenario{smoke, slow, untagged}

	names := func(in []*scenario.Scenario) []string {
		out := make([]string, 0, len(in))
		for _, s := range in {
			out = append(out, s.Name)
		}
		return out
	}

	tests := []struct {
		name     string
		include  []string
		exclude  []string
		expected []string
	}{
		{"no filters keeps everything", nil, nil, []string{"smoke", "slow", "untagged"}},
		{"include by tag", []string{"smoke"}, nil, []string{"smoke"}},
		{"exclude by tag", nil, []string{"regression"}, []string{"smoke", "untagged"}},
		{"exclusion applied after inclusion", []string{"smoke", "regression"}, []string{"fast"}, []string{"slow"}},
		{"untagged not matched by include", []string{"anything"}, nil, []string{}},
		{"no match yields empty", []string{"nope"}, nil, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := names(filterByTags(all, tc.include, tc.exclude))
			if strings.Join(got, ",") != strings.Join(tc.expected, ",") {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// --- 2D-4: error suggestions ---

func TestSuggestFor(t *testing.T) {
	tests := []struct{ msg, want string }{
		{"required flag(s) \"openapi\" not set", "Usage: kaktoos run --openapi <path> --env <path> --scenario <path>"},
		{"open missing.yml: no such file or directory", "Check the file path and ensure the file exists"},
		{"dial tcp 127.0.0.1:8080: connect: connection refused", "Check that the server at base_url is running and reachable"},
		{"unexpected status 401", "Check Authorization headers in your environment file"},
		{"unexpected status 429", "The server is rate limiting requests. Consider adding rate_limits to your environment file"},
		{"something else entirely", ""},
	}
	for _, tc := range tests {
		got := suggestFor(tc.msg)
		if tc.want == "" {
			if got != "" {
				t.Errorf("expected no suggestion for %q, got %q", tc.msg, got)
			}
			continue
		}
		if got != suggestionPrefix+tc.want {
			t.Errorf("for %q expected %q, got %q", tc.msg, suggestionPrefix+tc.want, got)
		}
	}
}

func TestPrintRunSuggestions(t *testing.T) {
	results := []engine.ExecutionResult{{
		Steps: []engine.StepResult{
			{Name: "ok", Status: engine.StepPassed},
			{Name: "refused", Status: engine.StepFailed, Error: "HTTP request failed: connection refused"},
			{Name: "unauthorized", Status: engine.StepFailed, StatusCode: 401},
		},
	}}

	var buf bytes.Buffer
	printRunSuggestions(results, &buf)
	out := buf.String()

	if !strings.Contains(out, "server at base_url is running") {
		t.Errorf("missing connection-refused suggestion: %q", out)
	}
	if !strings.Contains(out, "Authorization headers") {
		t.Errorf("missing 401 suggestion: %q", out)
	}
	if !strings.HasPrefix(out, suggestionPrefix) {
		t.Errorf("suggestions must use the %q prefix: %q", suggestionPrefix, out)
	}
}

func TestWithSuggestion_PreservesOriginalError(t *testing.T) {
	err := withSuggestion(os.ErrNotExist)
	if !strings.Contains(err.Error(), os.ErrNotExist.Error()) {
		t.Fatalf("original error text lost: %v", err)
	}
}

// --- 2D-1: enhanced validate ---

const validEnv = "base_url: http://localhost:9999\n"

const validOpenAPI = `
openapi: 3.0.3
info: {title: T, version: 1.0.0}
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        "200": {description: ok}
`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunValidation_ValidPhase2Scenario(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)
	scn := writeFile(t, dir, "scenario.yml", `
name: valid
steps:
  - name: list
    operation: listUsers
    timeout: 5s
    request:
      query:
        q: "{{toUpper(term)}}"
    retry:
      strategy: exponential_backoff
      max_attempts: 3
      initial_delay: 100ms
      backoff_multiplier: 2
      retry_on:
        status_codes: [503]
`)

	var out bytes.Buffer
	errs, warnings := runValidation(env, spec, []string{scn}, nil, &out)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestRunValidation_ClientErrorStatusCodeWarning(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)
	scn := writeFile(t, dir, "scenario.yml", `
name: warn
steps:
  - name: list
    operation: listUsers
    retry:
      strategy: fixed_delay
      max_attempts: 2
      initial_delay: 100ms
      retry_on:
        status_codes: [404]
`)

	var out bytes.Buffer
	errs, warnings := runValidation(env, spec, []string{scn}, nil, &out)
	if len(errs) != 0 {
		t.Fatalf("client-error codes must warn, not fail: %v", errs)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "client error status codes") {
		t.Fatalf("expected client-error warning, got %v", warnings)
	}
}

func TestRunValidation_UnknownTemplateFunctionIsError(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)
	scn := writeFile(t, dir, "scenario.yml", `
name: bad-template
steps:
  - name: list
    operation: listUsers
    request:
      query:
        q: "{{bogusFunc(term)}}"
`)

	var out bytes.Buffer
	errs, _ := runValidation(env, spec, []string{scn}, nil, &out)
	if len(errs) != 1 || !strings.Contains(errs[0], "unknown function") {
		t.Fatalf("expected unknown-function error, got %v", errs)
	}
}

func TestRunValidation_WrongArgCountIsError(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)
	scn := writeFile(t, dir, "scenario.yml", `
name: bad-args
steps:
  - name: list
    operation: listUsers
    request:
      query:
        q: "{{toUpper(a, b)}}"
`)

	var out bytes.Buffer
	errs, _ := runValidation(env, spec, []string{scn}, nil, &out)
	if len(errs) != 1 || !strings.Contains(errs[0], "argument") {
		t.Fatalf("expected argument-count error, got %v", errs)
	}
}

// --- 3A F3: inline scenarios ---

func TestRunValidation_InlineScenario(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)

	var out bytes.Buffer
	errs, warnings := runValidation(env, spec, nil, []string{"name: inline\nsteps:\n  - name: list\n    operation: listUsers\n"}, &out)
	if len(errs) != 0 || len(warnings) != 0 {
		t.Fatalf("expected a clean inline validation, got errs=%v warnings=%v", errs, warnings)
	}
	if !strings.Contains(out.String(), "inline scenario #1") {
		t.Fatalf("expected inline scenario reported in output, got %q", out.String())
	}
}

func TestRunValidation_BadInlineScenarioIsError(t *testing.T) {
	dir := t.TempDir()
	env := writeFile(t, dir, "env.yml", validEnv)
	spec := writeFile(t, dir, "openapi.yml", validOpenAPI)

	var out bytes.Buffer
	errs, _ := runValidation(env, spec, nil, []string{"steps: [[[garbage"}, &out)
	if len(errs) != 1 || !strings.Contains(errs[0], "inline scenario #1") {
		t.Fatalf("expected one inline scenario error, got %v", errs)
	}
}

func TestRunValidation_CollectsMultipleErrors(t *testing.T) {
	dir := t.TempDir()
	badEnv := writeFile(t, dir, "env.yml", "not_base_url: nope\n")
	badSpec := filepath.Join(dir, "missing-openapi.yml")
	badScn := writeFile(t, dir, "scenario.yml", `
name: bad
steps:
  - name: both
    operation: listUsers
    condition: "$.a equals 'b'"
`)

	var out bytes.Buffer
	errs, _ := runValidation(badEnv, badSpec, []string{badScn}, nil, &out)
	if len(errs) < 3 {
		t.Fatalf("expected all three failures collected, got %v", errs)
	}
}
