package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/kaktooslabs/kaktoos/internal/engine"
)

// suggestionPrefix is prepended to every hint, per requirement DX-4.
const suggestionPrefix = "  → "

// suggestFor returns a suggestion line for known error shapes, or "" when no
// suggestion applies. Exit codes are never affected by this.
func suggestFor(msg string) string {
	l := strings.ToLower(msg)

	switch {
	case strings.Contains(l, "required flag") || strings.Contains(l, "flag needs an argument"):
		return suggestionPrefix + "Usage: kaktoos run --openapi <path> --env <path> --scenario <path>"
	case strings.Contains(l, "no such file") || strings.Contains(l, "file not found"):
		return suggestionPrefix + "Check the file path and ensure the file exists"
	case strings.Contains(l, "connection refused"):
		return suggestionPrefix + "Check that the server at base_url is running and reachable"
	case strings.Contains(l, "401"):
		return suggestionPrefix + "Check Authorization headers in your environment file"
	case strings.Contains(l, "429"):
		return suggestionPrefix + "The server is rate limiting requests. Consider adding rate_limits to your environment file"
	}
	return ""
}

// withSuggestion appends a suggestion line to an error message when one applies.
// The error value and therefore the exit code are otherwise unchanged.
func withSuggestion(err error) error {
	s := suggestFor(err.Error())
	if s == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, s)
}

// printRunSuggestions writes hints for failures observed during execution
// (connection refused, HTTP 401, HTTP 429) after the text report.
func printRunSuggestions(results []engine.ExecutionResult, w io.Writer) {
	seen := map[string]bool{}
	emit := func(msg string) {
		if s := suggestFor(msg); s != "" && !seen[s] {
			seen[s] = true
			fmt.Fprintln(w, s)
		}
	}
	for _, r := range results {
		for _, step := range r.Steps {
			if step.Status == engine.StepPassed {
				continue
			}
			if step.Error != "" {
				emit(step.Error)
			}
			if step.StatusCode == 401 || step.StatusCode == 429 {
				emit(fmt.Sprintf("%d", step.StatusCode))
			}
		}
	}
}
