package reporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/engine"
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
