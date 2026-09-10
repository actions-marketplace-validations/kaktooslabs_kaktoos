package reporter

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/schema"
)

func TestExitCode_AllPassed(t *testing.T) {
	results := []engine.ExecutionResult{
		{Passed: true},
		{Passed: true},
	}
	code := ExitCode(results)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestExitCode_OneFailed(t *testing.T) {
	results := []engine.ExecutionResult{
		{Passed: true},
		{Passed: false},
	}
	code := ExitCode(results)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
}

func TestReport_BasicOutput(t *testing.T) {
	results := []engine.ExecutionResult{
		{
			ScenarioName: "test-scenario",
			Passed:       true,
			Steps: []engine.StepResult{
				{
					Name:   "step1",
					Status: engine.StepPassed,
				},
			},
		},
	}

	var buf bytes.Buffer
	Report(results, &buf)

	output := buf.String()
	if !strings.Contains(output, "test-scenario") {
		t.Error("output missing scenario name")
	}
	if !strings.Contains(output, "step1") {
		t.Error("output missing step name")
	}
	if !strings.Contains(output, "PASSED") {
		t.Error("output missing PASSED status")
	}
}

func TestReportJSON_ValidSchema(t *testing.T) {
	now := time.Now()
	results := []engine.ExecutionResult{
		{
			ExecutionID:  "exec-1",
			ScenarioName: "test-scenario",
			Passed:       true,
			Status:       engine.ExecutionPassed,
			StartedAt:    now,
			FinishedAt:   now.Add(time.Second),
			Duration:     time.Second,
			Steps: []engine.StepResult{
				{
					Name:       "step1",
					StepType:   "http",
					Status:     engine.StepPassed,
					Attempts:   []engine.AttemptResult{{Number: 1, Status: engine.AttemptPassed, StatusCode: 200, Duration: time.Millisecond}},
					Assertions: []engine.AssertionResult{{Type: "status", Passed: true}},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := ReportJSON(results, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded jsonReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Summary.Total != 1 || decoded.Summary.Passed != 1 {
		t.Fatalf("unexpected summary: %+v", decoded.Summary)
	}
	if decoded.Executions[0].ExecutionID != "exec-1" {
		t.Fatalf("expected execution_id in output, got %+v", decoded.Executions[0])
	}
	if decoded.Executions[0].Steps[0].Attempts[0].StatusCode != 200 {
		t.Fatalf("expected attempt data in output: %+v", decoded.Executions[0].Steps[0])
	}
}

func TestReportJUnit_ValidXML(t *testing.T) {
	results := []engine.ExecutionResult{
		{
			ScenarioName: "suite-1",
			Duration:     time.Second,
			Steps: []engine.StepResult{
				{Name: "ok-step", Status: engine.StepPassed, Duration: 100 * time.Millisecond},
				{
					Name: "failed-step", Status: engine.StepFailed, Duration: 50 * time.Millisecond,
					Error:      "assertion failed: body.equals $.id: expected '123' got '456'",
					Assertions: []engine.AssertionResult{{Type: "equals", Passed: false}},
				},
				{
					Name: "errored-step", Status: engine.StepFailed, Duration: 20 * time.Millisecond,
					Error: "variable 'missing' not found",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := ReportJUnit(results, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded junitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, buf.String())
	}
	suite := decoded.Suites[0]
	if suite.Tests != 3 || suite.Failures != 1 || suite.Errors != 1 {
		t.Fatalf("unexpected classification: tests=%d failures=%d errors=%d", suite.Tests, suite.Failures, suite.Errors)
	}
	if suite.Cases[1].Failure == nil {
		t.Fatal("expected assertion failure classified as <failure>")
	}
	if suite.Cases[2].Error == nil {
		t.Fatal("expected variable error classified as <error>")
	}
}

func TestReportJSON_SchemaViolationsAndUndeclaredFields(t *testing.T) {
	results := []engine.ExecutionResult{
		{
			ScenarioName: "schema-test",
			Passed:       false,
			Steps: []engine.StepResult{
				{
					Name:   "step1",
					Status: engine.StepFailed,
					SchemaViolations: []schema.Violation{
						{Kind: schema.KindRequiredFieldMissing, Path: "$.name", Message: "required field is missing"},
					},
					UndeclaredFields: []string{"$.extra"},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := ReportJSON(results, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded jsonReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	step := decoded.Executions[0].Steps[0]
	if len(step.SchemaViolations) != 1 || step.SchemaViolations[0].Kind != schema.KindRequiredFieldMissing {
		t.Fatalf("expected schema violation in output, got %+v", step)
	}
	if len(step.UndeclaredFields) != 1 || step.UndeclaredFields[0] != "$.extra" {
		t.Fatalf("expected undeclared field in output, got %+v", step)
	}
}

func TestReportJUnit_SchemaViolationClassifiedAsFailure(t *testing.T) {
	results := []engine.ExecutionResult{{
		ScenarioName: "suite",
		Steps: []engine.StepResult{
			{
				Name: "get", Status: engine.StepFailed,
				Error: "schema violation: required field is missing",
				SchemaViolations: []schema.Violation{
					{Kind: schema.KindRequiredFieldMissing, Path: "$.name", Message: "required field is missing"},
				},
			},
		},
	}}
	var buf bytes.Buffer
	if err := ReportJUnit(results, &buf); err != nil {
		t.Fatal(err)
	}
	var decoded junitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Suites[0].Failures != 1 || decoded.Suites[0].Errors != 0 {
		t.Fatalf("expected schema violation classified as failure, got failures=%d errors=%d", decoded.Suites[0].Failures, decoded.Suites[0].Errors)
	}
	if !strings.Contains(decoded.Suites[0].Cases[0].Failure.Text, "required_field_missing") {
		t.Fatalf("expected violation kind in failure text, got %q", decoded.Suites[0].Cases[0].Failure.Text)
	}
}

func TestReportJUnit_ConditionFalseIsFailure(t *testing.T) {
	results := []engine.ExecutionResult{{
		ScenarioName: "suite",
		Steps: []engine.StepResult{
			{Name: "cond", Status: engine.StepFailed, StepType: "condition", Error: "condition evaluated to false: $.active equals 'yes'"},
		},
	}}
	var buf bytes.Buffer
	if err := ReportJUnit(results, &buf); err != nil {
		t.Fatal(err)
	}
	var decoded junitTestSuites
	xml.Unmarshal(buf.Bytes(), &decoded)
	if decoded.Suites[0].Failures != 1 || decoded.Suites[0].Errors != 0 {
		t.Fatalf("expected condition-false classified as failure, got failures=%d errors=%d", decoded.Suites[0].Failures, decoded.Suites[0].Errors)
	}
}
