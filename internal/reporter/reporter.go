package reporter

import (
	"fmt"
	"io"

	"github.com/kaktooslabs/kaktoos/internal/engine"
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
