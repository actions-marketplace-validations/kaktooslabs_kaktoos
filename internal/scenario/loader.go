package scenario

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads a Scenario from a YAML data source, strictly validating it against known fields.
// It returns an error if unknown or disallowed fields are found, or if unmarshalling fails.
func Load(data []byte) (*Scenario, error) {
	// Pass 1: Unmarshal into a map[string]interface{} to detect unknown fields.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal step data into map: %w", err)
	}

	var errs []string
	allowedAtScenario := map[string]bool{"name": true, "steps": true, "tags": true, "workflow_timeout": true, "idempotency_key": true}
	for k := range raw {
		if !allowedAtScenario[k] {
			errs = append(errs, fmt.Sprintf("unknown field in Scenario: %s", k))
		}
	}

	// If 'steps' is present, we need to check the step items within it.
	stepsRaw, ok := raw["steps"].([]interface{})
	if ok {
		// Updated: Step must now reference assertion.Spec
		stepAllowedFields := map[string]bool{"name": true, "operation": true, "request": true, "extract": true, "assert": true, "condition": true, "timeout": true, "retry": true, "always_run": true}
		for i, stepRaw := range stepsRaw {
			m, ok := stepRaw.(map[string]interface{})
			if !ok {
				errs = append(errs, fmt.Sprintf("step %d is not a valid map structure", i))
				continue
			}

			for k := range m {
				if !stepAllowedFields[k] {
					errs = append(errs, fmt.Sprintf("unknown field in Scenario step %d: %s", i, k))
				}
			}
			// Note: Further recursive validation for nested structs (RequestSpec, AssertSpec)
			// would be required but is complex for initial scope. We prioritize top-level checks.
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	// Pass 2: Unmarshal into the typed struct.
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scenario into struct: %w", err)
	}

	if s.WorkflowTimeout != "" {
		if _, err := time.ParseDuration(s.WorkflowTimeout); err != nil {
			errs = append(errs, fmt.Sprintf("scenario: invalid workflow_timeout %q: %v", s.WorkflowTimeout, err))
		}
	}
	for _, step := range s.Steps {
		prefix := fmt.Sprintf("scenario: step %q:", step.Name)
		if (step.Operation == "") == (step.Condition == "") {
			errs = append(errs, prefix+" must have exactly one of 'operation' and 'condition'")
		}
		if step.Condition != "" && !validCondition(step.Condition) {
			errs = append(errs, fmt.Sprintf("%s invalid condition syntax %q", prefix, step.Condition))
		}
		if step.Condition != "" && step.Retry != nil {
			errs = append(errs, prefix+" condition steps cannot have a retry policy")
		}
		if step.Condition != "" && step.AlwaysRun {
			errs = append(errs, prefix+" always_run is only valid on operation steps")
		}
		if step.Timeout != "" {
			if _, err := time.ParseDuration(step.Timeout); err != nil {
				errs = append(errs, fmt.Sprintf("%s invalid timeout %q: %v", prefix, step.Timeout, err))
			}
		}
		if retry := step.Retry; retry != nil {
			if retry.Strategy != "fixed_delay" && retry.Strategy != "exponential_backoff" && retry.Strategy != "linear_backoff" {
				errs = append(errs, prefix+" retry: strategy: invalid")
			}
			if retry.MaxAttempts < 1 || retry.MaxAttempts > 10 {
				errs = append(errs, prefix+" retry: max_attempts: must be 1-10")
			}
			d, e := time.ParseDuration(retry.InitialDelay)
			if e != nil || d < 10*time.Millisecond {
				errs = append(errs, prefix+" retry: initial_delay: must be at least 10ms")
			}
			if retry.MaxDelay != "" {
				if _, e := time.ParseDuration(retry.MaxDelay); e != nil {
					errs = append(errs, prefix+" retry: max_delay: invalid")
				}
			}
			if retry.Strategy == "exponential_backoff" && retry.BackoffMultiplier <= 0 {
				errs = append(errs, prefix+" retry: backoff_multiplier: must be greater than zero")
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return &s, nil
}

func validCondition(condition string) bool {
	fields := strings.Fields(condition)
	if len(fields) < 2 || len(fields) > 3 {
		return false
	}
	switch fields[1] {
	case "exists", "not_exists":
		return len(fields) == 2
	case "equals", "not_equals", "greater_than", "less_than", "greater_than_or_equal", "less_than_or_equal":
		return len(fields) == 3
	}
	return false
}

// We assume the other loaders (Config, OpenAPI) follow a similar strict loading pattern.
// This loader serves as the blueprint for strict YAML loading.
