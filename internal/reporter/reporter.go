package reporter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/verification"
)

// Report formats and writes test results to io.Writer.
func Report(results []engine.ExecutionResult, w io.Writer) {
	for _, result := range results {
		fmt.Fprintf(w, "\n=== Scenario: %s ===\n", result.ScenarioName)

		for _, step := range result.Steps {
			fmt.Fprintf(w, "  [%s] %s\n", step.Status, step.Name)

			if step.Status == engine.StepFailed {
				if step.Error != "" {
					fmt.Fprintf(w, "    Error: %s\n", step.Error)
				}
				if step.FailureCategory != "" {
					fmt.Fprintf(w, "    Category: %s\n", step.FailureCategory)
				}
				for _, assertion := range step.Assertions {
					if !assertion.Passed {
						fmt.Fprintf(w, "    Assertion failed: %s\n", assertion.Type)
						fmt.Fprintf(w, "      Expected: %v\n", assertion.Expected)
						fmt.Fprintf(w, "      Actual: %v\n", assertion.Actual)
						if assertion.Error != "" {
							fmt.Fprintf(w, "      Error: %s\n", assertion.Error)
						}
					}
				}
			}
		}

		if result.Passed {
			fmt.Fprintf(w, "✓ Scenario PASSED\n")
		} else {
			fmt.Fprintf(w, "✗ Scenario FAILED\n")
		}
	}
}

// ExitCode determines the process exit code from the results slice.
// Returns 0 if all results passed, 1 otherwise.
func ExitCode(results []engine.ExecutionResult) int {
	// Check if any result has Passed == false
	for _, result := range results {
		if !result.Passed {
			return 1
		}
	}
	// All passed
	return 0
}

// --- JSON output (DX-3) ---

type jsonReport struct {
	Summary    jsonSummary     `json:"summary"`
	Executions []jsonExecution `json:"executions"`
}

type jsonSummary struct {
	Total     int    `json:"total"`
	Passed    int    `json:"passed"`
	Failed    int    `json:"failed"`
	TimedOut  int    `json:"timed_out"`
	Duration  string `json:"duration"`
	StartTime string `json:"start_time"`
}

type jsonExecution struct {
	ExecutionID string     `json:"execution_id"`
	Workflow    string     `json:"workflow"`
	Status      string     `json:"status"`
	StartedAt   string     `json:"started_at"`
	FinishedAt  string     `json:"finished_at"`
	Duration    string     `json:"duration"`
	Steps       []jsonStep `json:"steps"`
}

type jsonStep struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Status        string            `json:"status"`
	StartedAt     string            `json:"started_at"`
	FinishedAt    string            `json:"finished_at"`
	Duration      string            `json:"duration"`
	Attempts      []jsonAttempt     `json:"attempts,omitempty"`
	Assertions    []jsonAssertion   `json:"assertions,omitempty"`
	ExtractedVars map[string]string `json:"extracted_vars,omitempty"`
	// AssertionFailures is the failed-only view of Assertions, so a consumer
	// (CI, or an AI agent) can see what broke without filtering.
	AssertionFailures []jsonAssertion       `json:"assertion_failures,omitempty"`
	SchemaViolations  []jsonSchemaViolation `json:"schema_violations,omitempty"`
	UndeclaredFields  []string              `json:"undeclared_fields,omitempty"`
	// FailureCategory is the engine's deterministic failure class; empty when passed.
	FailureCategory string `json:"failure_category,omitempty"`
	// Evidence is the redacted request/response behind a failure.
	Evidence *verification.Evidence `json:"evidence,omitempty"`
}

type jsonSchemaViolation struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type jsonAttempt struct {
	Number     int    `json:"number"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Duration   string `json:"duration"`
}

type jsonAssertion struct {
	Type   string `json:"type"`
	Passed bool   `json:"passed"`
}

// ReportJSON writes results as the DX-3 JSON schema.
func ReportJSON(results []engine.ExecutionResult, w io.Writer) error {
	report := jsonReport{Executions: make([]jsonExecution, 0, len(results))}
	report.Summary.Total = len(results)

	var totalDuration time.Duration
	var start time.Time
	for i, r := range results {
		if r.Passed {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
		if r.Status == engine.ExecutionTimedOut {
			report.Summary.TimedOut++
		}
		totalDuration += r.Duration
		if i == 0 || r.StartedAt.Before(start) {
			start = r.StartedAt
		}

		exec := jsonExecution{
			ExecutionID: r.ExecutionID,
			Workflow:    r.ScenarioName,
			Status:      string(r.Status),
			StartedAt:   formatRFC3339(r.StartedAt),
			FinishedAt:  formatRFC3339(r.FinishedAt),
			Duration:    r.Duration.String(),
			Steps:       make([]jsonStep, 0, len(r.Steps)),
		}
		for _, s := range r.Steps {
			step := jsonStep{
				Name:          s.Name,
				Type:          s.StepType,
				Status:        string(s.Status),
				StartedAt:     formatRFC3339(s.StartedAt),
				FinishedAt:    formatRFC3339(s.FinishedAt),
				Duration:      s.Duration.String(),
				ExtractedVars: s.ExtractedVars,
			}
			for _, a := range s.Attempts {
				step.Attempts = append(step.Attempts, jsonAttempt{
					Number: a.Number, Status: string(a.Status), StatusCode: a.StatusCode, Duration: a.Duration.String(),
				})
			}
			for _, as := range s.Assertions {
				step.Assertions = append(step.Assertions, jsonAssertion{Type: as.Type, Passed: as.Passed})
				if !as.Passed {
					step.AssertionFailures = append(step.AssertionFailures, jsonAssertion{Type: as.Type, Passed: false})
				}
			}
			for _, v := range s.SchemaViolations {
				step.SchemaViolations = append(step.SchemaViolations, jsonSchemaViolation{Kind: v.Kind, Path: v.Path, Message: v.Message})
			}
			step.UndeclaredFields = s.UndeclaredFields
			step.FailureCategory = string(s.FailureCategory)
			step.Evidence = s.Evidence
			exec.Steps = append(exec.Steps, step)
		}
		report.Executions = append(report.Executions, exec)
	}
	report.Summary.Duration = totalDuration.String()
	report.Summary.StartTime = formatRFC3339(start)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// --- JUnit output (DX-3) ---

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Errors   int             `xml:"errors,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name    string        `xml:"name,attr"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
	Error   *junitError   `xml:"error,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type junitError struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// ReportJUnit writes results as xunit-style JUnit XML. A step is a "failure" when
// assertions failed or a condition evaluated false; it is an "error" for any other
// non-passing status (network, timeout, variable/template errors). Classification
// uses StepResult.Status and StepResult.Error content, never StatusCode == 0.
func ReportJUnit(results []engine.ExecutionResult, w io.Writer) error {
	out := junitTestSuites{}
	for _, r := range results {
		suite := junitTestSuite{Name: r.ScenarioName, Time: formatSeconds(r.Duration)}
		for _, s := range r.Steps {
			suite.Tests++
			tc := junitTestCase{Name: s.Name, Time: formatSeconds(s.Duration)}

			if s.Status != engine.StepPassed && s.Status != engine.StepSkipped {
				if isAssertionFailure(s) {
					suite.Failures++
					tc.Failure = &junitFailure{Message: s.Error, Text: failureText(s)}
				} else {
					suite.Errors++
					tc.Error = &junitError{Message: s.Error, Text: s.Error}
				}
			}
			suite.Cases = append(suite.Cases, tc)
		}
		out.Suites = append(out.Suites, suite)
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(out)
}

// failureText is the <failure> body: the step error plus every schema
// violation, so multiple violations are not lost behind the single-line
// message attribute.
func failureText(s engine.StepResult) string {
	parts := make([]string, 0, 1+len(s.SchemaViolations))
	if s.Error != "" {
		parts = append(parts, s.Error)
	}
	for _, v := range s.SchemaViolations {
		if v.Path != "" {
			parts = append(parts, fmt.Sprintf("%s %s: %s", v.Kind, v.Path, v.Message))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %s", v.Kind, v.Message))
		}
	}
	return strings.Join(parts, "\n")
}

// isAssertionFailure reports whether a non-passing step failed due to a failed
// assertion or a false condition, rather than an infrastructure error. It
// reads the engine's FailureCategory when present; the string heuristics below
// only cover legacy results without one (e.g. rebuilt from idempotency records).
func isAssertionFailure(s engine.StepResult) bool {
	switch s.FailureCategory {
	case verification.CategoryAssertionFailed, verification.CategoryContractMismatch:
		return true
	case "":
		// fall through to legacy heuristics
	default:
		return false
	}
	if s.StepType == "condition" {
		return true
	}
	if len(s.SchemaViolations) > 0 {
		return true
	}
	for _, a := range s.Assertions {
		if !a.Passed {
			return true
		}
	}
	if s.Error != "" {
		el := strings.ToLower(s.Error)
		if strings.Contains(el, "assertion") {
			return true
		}
	}
	return false
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}
