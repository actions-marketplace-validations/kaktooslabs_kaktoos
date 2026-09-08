package engine

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kaktooslabs/kaktoos/internal/assertion"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/variable"
)

// RunScenario performs the full execution of a scenario against a target service.
// It creates a new variable store, executes each step in order, and stops on first failure.
// All subsequent steps after a failure are marked as StepSkipped.
func RunScenario(ctx ExecutionContext, s scenario.Scenario, client *http.Client) ExecutionResult {
	executionResult := ExecutionResult{
		ScenarioName: s.Name,
		Steps:        make([]StepResult, 0, len(s.Steps)),
		Passed:       true,
	}

	stepFailed := false

	for _, step := range s.Steps {
		// Rule 6: If previous step failed, skip all remaining steps
		if stepFailed {
			stepResult := StepResult{
				Name:   step.Name,
				Status: StepSkipped,
				Error:  "Skipped due to previous step failure",
			}
			executionResult.Steps = append(executionResult.Steps, stepResult)
			continue
		}

		stepResult := executeStep(ctx, step, client)
		executionResult.Steps = append(executionResult.Steps, stepResult)

		if stepResult.Status == StepFailed {
			stepFailed = true
			executionResult.Passed = false
		}
	}

	return executionResult
}

// executeStep runs a single step: resolves operation, builds request, executes HTTP,
// extracts variables, evaluates assertions.
func executeStep(ctx ExecutionContext, step scenario.Step, client *http.Client) StepResult {
	stepResult := StepResult{
		Name:       step.Name,
		Status:     StepPassed,
		Assertions: make([]AssertionResult, 0),
	}

	// 1. Find the operation in the OpenAPI spec
	// The operation name refers to the operationId in the OpenAPI spec
	var foundOp *openapi.Operation
	var foundPath string
	var foundMethod string

	// Search through all paths and methods to find the operation by ID
	for path, opMap := range ctx.Operations {
		for method, op := range opMap {
			if op.Name == step.Operation {
				foundOp = &op
				foundPath = path
				foundMethod = method
				break
			}
		}
		if foundOp != nil {
			break
		}
	}

	if foundOp == nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("operation %s not found in OpenAPI spec", step.Operation)
		return stepResult
	}

	// 2. Build the request
	// Substitute path parameters if provided
	pathTemplate := foundPath
	if step.Request != nil && len(step.Request.Path) > 0 {
		for key, val := range step.Request.Path {
			// Substitute variables in the value first
			substituted, err := variable.Substitute(val, ctx.VariableStore, step.Name)
			if err != nil {
				stepResult.Status = StepFailed
				stepResult.Error = fmt.Sprintf("path param substitution failed: %v", err)
				return stepResult
			}
			pathTemplate = strings.ReplaceAll(pathTemplate, "{"+key+"}", substituted)
		}
	}

	// Prepare request spec
	reqSpec := httpclient.RequestSpec{
		Method:       foundMethod,
		BaseURL:      ctx.Environment.BaseURL,
		PathTemplate: pathTemplate,
		PathParams:   make(map[string]string), // Already substituted above
		BaseHeaders:  ctx.Environment.Headers,
	}

	if step.Request != nil {
		// Substitute query parameters
		if len(step.Request.Query) > 0 {
			reqSpec.Query = make(map[string]string)
			for k, v := range step.Request.Query {
				substituted, err := variable.Substitute(v, ctx.VariableStore, step.Name)
				if err != nil {
					stepResult.Status = StepFailed
					stepResult.Error = fmt.Sprintf("query param substitution failed: %v", err)
					return stepResult
				}
				reqSpec.Query[k] = substituted
			}
		}

		// Substitute headers
		if len(step.Request.Headers) > 0 {
			reqSpec.Headers = make(map[string]string)
			for k, v := range step.Request.Headers {
				substituted, err := variable.Substitute(v, ctx.VariableStore, step.Name)
				if err != nil {
					stepResult.Status = StepFailed
					stepResult.Error = fmt.Sprintf("header substitution failed: %v", err)
					return stepResult
				}
				reqSpec.Headers[k] = substituted
			}
		}

		// Substitute body
		if step.Request.Body != "" {
			substituted, err := variable.Substitute(step.Request.Body, ctx.VariableStore, step.Name)
			if err != nil {
				stepResult.Status = StepFailed
				stepResult.Error = fmt.Sprintf("body substitution failed: %v", err)
				return stepResult
			}
			reqSpec.Body = []byte(substituted)
		}
	}

	req, err := httpclient.BuildRequest(reqSpec)
	if err != nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("build request failed: %v", err)
		return stepResult
	}

	// 3. Execute the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("HTTP request failed: %v", err)
		return stepResult
	}
	defer resp.Body.Close()

	stepResult.StatusCode = resp.StatusCode

	// 4. Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("failed to read response body: %v", err)
		return stepResult
	}

	// 5. Extract variables
	if step.Extract != nil && len(step.Extract) > 0 {
		err := variable.Extract(bodyBytes, step.Extract, ctx.VariableStore, step.Name)
		if err != nil {
			stepResult.Status = StepFailed
			stepResult.Error = fmt.Sprintf("variable extraction failed: %v", err)
			return stepResult
		}
	}

	// 6. Evaluate assertions
	if step.Assert != nil {
		// Convert scenario.AssertSpec to assertion.Spec
		assertSpec := assertion.Spec{
			Status: step.Assert.Status,
			Body:   make([]assertion.BodyAssert, len(step.Assert.Body)),
		}
		for i, ba := range step.Assert.Body {
			// Substitute variables in Equals value if it's a string
			equalsValue := ba.Equals
			if strVal, ok := ba.Equals.(string); ok {
				substituted, err := variable.Substitute(strVal, ctx.VariableStore, step.Name)
				if err != nil {
					stepResult.Status = StepFailed
					stepResult.Error = fmt.Sprintf("assertion equals substitution failed: %v", err)
					return stepResult
				}
				equalsValue = substituted
			}

			assertSpec.Body[i] = assertion.BodyAssert{
				Path:      ba.Path,
				Equals:    equalsValue,
				Exists:    ba.Exists,
				NotExists: ba.NotExists,
			}
		}

		// Create a mock response with the body already read
		mockResp := &http.Response{
			StatusCode: resp.StatusCode,
			Body:       io.NopCloser(strings.NewReader(string(bodyBytes))),
		}

		assertionResults := assertion.Evaluate(mockResp, assertSpec)

		// Convert assertion.Result to AssertionResult
		for _, ar := range assertionResults {
			stepResult.Assertions = append(stepResult.Assertions, AssertionResult{
				Type:     ar.Type,
				Path:     ar.Path,
				Expected: ar.Expected,
				Actual:   ar.Actual,
				Passed:   ar.Passed,
				Error:    ar.Error,
			})

			// If any assertion failed, mark step as failed
			if !ar.Passed {
				stepResult.Status = StepFailed
				if stepResult.Error == "" {
					stepResult.Error = fmt.Sprintf("assertion failed: %s", ar.Error)
				}
			}
		}
	}

	return stepResult
}
