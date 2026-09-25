package reporter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/verification"
)

func failedStep(cat verification.Category) engine.ExecutionResult {
	return engine.ExecutionResult{
		ScenarioName: "s", Status: engine.ExecutionFailed,
		Steps: []engine.StepResult{{
			Name: "read back", StepType: "http", Status: engine.StepFailed,
			StatusCode: 404, Error: "assertion failed", FailureCategory: cat,
			Evidence: verification.NewEvidence("GET", "http://x/orders/1", 404,
				map[string]string{"Authorization": "Bearer s3cret"}, "",
				map[string]string{"Content-Type": "application/json"}, `{"error":"not found"}`),
		}},
	}
}

func TestReportJSON_CarriesCategoryAndRedactedEvidence(t *testing.T) {
	var buf bytes.Buffer
	if err := ReportJSON([]engine.ExecutionResult{failedStep(verification.CategoryAssertionFailed)}, &buf); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Executions []struct {
			Steps []struct {
				FailureCategory string                 `json:"failure_category"`
				Evidence        *verification.Evidence `json:"evidence"`
			} `json:"steps"`
		} `json:"executions"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	s := got.Executions[0].Steps[0]
	if s.FailureCategory != string(verification.CategoryAssertionFailed) {
		t.Fatalf("failure_category = %q", s.FailureCategory)
	}
	if s.Evidence == nil || s.Evidence.RequestHeaders["Authorization"] != "***" {
		t.Fatalf("evidence missing or unredacted: %+v", s.Evidence)
	}
	if s.Evidence.ResponseBody != `{"error":"not found"}` {
		t.Fatalf("response body not carried: %+v", s.Evidence)
	}
}

func TestReportJSON_PassingRunEmitsNeitherKey(t *testing.T) {
	var buf bytes.Buffer
	pass := engine.ExecutionResult{
		ScenarioName: "s", Passed: true, Status: engine.ExecutionPassed,
		Steps: []engine.StepResult{{Name: "get", StepType: "http", Status: engine.StepPassed, StatusCode: 200}},
	}
	if err := ReportJSON([]engine.ExecutionResult{pass}, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "failure_category") || strings.Contains(buf.String(), "evidence") {
		t.Fatalf("passing run must not emit verification keys:\n%s", buf.String())
	}
}

func TestReportJUnit_RoutesByCategory(t *testing.T) {
	cases := map[verification.Category]string{
		verification.CategoryAssertionFailed:  "<failure",
		verification.CategoryContractMismatch: "<failure",
		verification.CategoryAuthFailure:      "<error",
		verification.CategoryTransportFailure: "<error",
		verification.CategoryTimeout:          "<error",
		verification.CategoryRateLimited:      "<error",
		verification.CategoryServerError:      "<error",
		verification.CategoryConfigError:      "<error",
	}
	for cat, want := range cases {
		var buf bytes.Buffer
		if err := ReportJUnit([]engine.ExecutionResult{failedStep(cat)}, &buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), want) {
			t.Errorf("category %s: expected %s element, got:\n%s", cat, want, buf.String())
		}
	}
}

func TestReport_TextShowsCategory(t *testing.T) {
	var buf bytes.Buffer
	Report([]engine.ExecutionResult{failedStep(verification.CategoryAuthFailure)}, &buf)
	if !strings.Contains(buf.String(), "Category: auth_failure") {
		t.Fatalf("text report missing category:\n%s", buf.String())
	}
}
