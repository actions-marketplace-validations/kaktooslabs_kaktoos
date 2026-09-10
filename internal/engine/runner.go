package engine

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kaktooslabs/kaktoos/internal/assertion"
	"github.com/kaktooslabs/kaktoos/internal/httpclient"
	"github.com/kaktooslabs/kaktoos/internal/idempotency"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/kaktooslabs/kaktoos/internal/schema"
	"github.com/kaktooslabs/kaktoos/internal/template"
	"github.com/kaktooslabs/kaktoos/internal/variable"
)

// resolve applies the template function pre-pass (template.Resolve) before
// plain {{var}} substitution, per the frozen engine contract.
func resolve(s string, store *variable.Store, stepName string) (string, error) {
	s, err := template.Resolve(s, store, stepName)
	if err != nil {
		return "", err
	}
	return variable.Substitute(s, store, stepName)
}

// flattenHeader collapses an http.Header into single-valued form for tracing.
func flattenHeader(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

// truncate caps a traced body at n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// RunScenario performs the full execution of a scenario against a target service.
// It creates a new variable store, executes each step in order, and stops on first failure.
// All subsequent steps after a failure are marked as StepSkipped.
func RunScenario(ctx ExecutionContext, s scenario.Scenario, client *http.Client) ExecutionResult {
	executionResult := ExecutionResult{
		ExecutionID:  uuid.New().String(),
		StartedAt:    time.Now(),
		ScenarioName: s.Name,
		Steps:        make([]StepResult, 0, len(s.Steps)),
		Passed:       true,
	}
	var key string
	if ctx.Idempotency != nil && s.IdempotencyKey != "" {
		var err error
		key, err = variable.Substitute(s.IdempotencyKey, ctx.VariableStore, "idempotency_key")
		if err != nil {
			executionResult.Passed = false
			executionResult.Status = ExecutionFailed
			executionResult.Error = "idempotency: " + err.Error()
			return executionResult
		}
		if record, _ := ctx.Idempotency.Get(key); record != nil && record.Status == idempotency.StatusCompleted {
			return rebuildExecutionResult(*record)
		}
		_ = ctx.Idempotency.Set(key, idempotency.Record{Key: key, Status: idempotency.StatusInProgress})
	}
	runCtx := context.Background()
	var cancel context.CancelFunc
	if s.WorkflowTimeout != "" {
		if d, err := time.ParseDuration(s.WorkflowTimeout); err == nil {
			runCtx, cancel = context.WithTimeout(runCtx, d)
			defer cancel()
		}
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

		var stepResult StepResult
		if step.Condition != "" {
			stepResult = executeConditionStep(ctx, step)
		} else {
			stepResult = executeStep(ctx, step, client, runCtx)
			stepResult.StepType = "http"
		}
		executionResult.Steps = append(executionResult.Steps, stepResult)

		if stepResult.Status == StepFailed || stepResult.Status == StepTimedOut {
			stepFailed = true
			executionResult.Passed = false
		}
	}
	executionResult.FinishedAt = time.Now()
	executionResult.Duration = executionResult.FinishedAt.Sub(executionResult.StartedAt)
	if executionResult.Passed {
		executionResult.Status = ExecutionPassed
	} else {
		executionResult.Status = ExecutionFailed
		for _, st := range executionResult.Steps {
			if st.Status == StepTimedOut {
				executionResult.Status = ExecutionTimedOut
			}
		}
	}
	if key != "" {
		_ = ctx.Idempotency.Set(key, buildIdempotencyRecord(key, executionResult))
	}
	return executionResult
}

func executeConditionStep(ctx ExecutionContext, step scenario.Step) StepResult {
	r := StepResult{Name: step.Name, StepType: "condition", Status: StepPassed, Assertions: []AssertionResult{}, Attempts: []AttemptResult{}, StartedAt: time.Now()}
	defer func() { r.FinishedAt = time.Now(); r.Duration = r.FinishedAt.Sub(r.StartedAt) }()
	f := strings.Fields(step.Condition)
	if len(f) < 2 || len(f) > 3 {
		r.Status = StepFailed
		r.Error = "condition error: invalid syntax"
		return r
	}
	op := f[1]
	unary := op == "exists" || op == "not_exists"
	if unary != (len(f) == 2) {
		r.Status = StepFailed
		r.Error = "condition error: invalid syntax"
		return r
	}
	resolve := func(x string) (string, bool) {
		if strings.HasPrefix(x, "$.") {
			x = x[2:]
		} else if strings.HasPrefix(x, "$") {
			x = x[1:]
		} else {
			return strings.Trim(x, "'"), true
		}
		v, ok := ctx.VariableStore.Get(x)
		return v, ok
	}
	lhs, found := resolve(f[0])
	if unary {
		ok := found
		if op == "not_exists" {
			ok = !ok
		}
		if !ok {
			r.Status = StepFailed
			r.Error = "condition evaluated to false: " + step.Condition
		}
		return r
	}
	if !found {
		r.Status = StepFailed
		r.Error = fmt.Sprintf("condition error: variable '%s' not found", strings.TrimPrefix(strings.TrimPrefix(f[0], "$."), "$"))
		return r
	}
	rhs, _ := resolve(f[2])
	var ok bool
	switch op {
	case "equals":
		ok = lhs == rhs
	case "not_equals":
		ok = lhs != rhs
	case "greater_than", "less_than", "greater_than_or_equal", "less_than_or_equal":
		a, e := strconv.ParseFloat(lhs, 64)
		b, e2 := strconv.ParseFloat(rhs, 64)
		if e != nil || e2 != nil {
			r.Status = StepFailed
			r.Error = "condition error: operand is not numeric"
			return r
		}
		if op == "greater_than" {
			ok = a > b
		} else if op == "less_than" {
			ok = a < b
		} else if op == "greater_than_or_equal" {
			ok = a >= b
		} else {
			ok = a <= b
		}
	default:
		r.Status = StepFailed
		r.Error = "condition error: invalid operator"
		return r
	}
	if !ok {
		r.Status = StepFailed
		r.Error = "condition evaluated to false: " + step.Condition
	}
	return r
}

// executeStep runs a single step: resolves operation, builds request, executes HTTP,
// extracts variables, evaluates assertions.
func executeStep(ctx ExecutionContext, step scenario.Step, client *http.Client, parent context.Context) StepResult {
	stepCtx := parent
	var cancel context.CancelFunc
	if step.Timeout != "" {
		if d, e := time.ParseDuration(step.Timeout); e == nil {
			stepCtx, cancel = context.WithTimeout(parent, d)
			defer cancel()
		}
	}
	max := 1
	if step.Retry != nil {
		max = step.Retry.MaxAttempts
	}
	var final StepResult
	var attempts []AttemptResult
	for n := 1; n <= max; n++ {
		if ctx.RateLimiter != nil {
			if e := ctx.RateLimiter.Wait(stepCtx, step.Operation); e != nil {
				return timeoutResult(step, attempts, e)
			}
		}
		final = executeSingleStep(ctx, step, client, stepCtx)
		a := final.Attempts[0]
		a.Number = n
		attempts = append(attempts, a)
		final.Attempts = attempts
		if stepCtx.Err() != nil {
			return timeoutResult(step, attempts, stepCtx.Err())
		}
		if !retryable(step.Retry, a) || n == max {
			return final
		}
		d := retryDelay(step.Retry, n)
		select {
		case <-time.After(d):
		case <-stepCtx.Done():
			return timeoutResult(step, attempts, stepCtx.Err())
		}
	}
	return final
}
func timeoutResult(s scenario.Step, a []AttemptResult, e error) StepResult {
	return StepResult{Name: s.Name, StepType: "http", Status: StepTimedOut, Error: "engine: step timed out: " + e.Error(), Attempts: a, StartedAt: time.Now(), FinishedAt: time.Now()}
}
func retryable(p *scenario.RetryPolicy, a AttemptResult) bool {
	if p == nil {
		return false
	}
	if a.Status == AttemptFailed {
		return a.StatusCode == 0 && p.RetryOn.NetworkErrors
	}
	for _, c := range p.RetryOn.StatusCodes {
		if c == a.StatusCode {
			return true
		}
	}
	return false
}
func retryDelay(p *scenario.RetryPolicy, n int) time.Duration {
	d, _ := time.ParseDuration(p.InitialDelay)
	switch p.Strategy {
	case "linear_backoff":
		d *= time.Duration(n)
	case "exponential_backoff":
		for i := 1; i < n; i++ {
			d = time.Duration(float64(d) * p.BackoffMultiplier)
		}
	}
	if p.MaxDelay != "" {
		if m, e := time.ParseDuration(p.MaxDelay); e == nil && d > m {
			d = m
		}
	}
	return d
}

func buildIdempotencyRecord(key string, result ExecutionResult) idempotency.Record {
	record := idempotency.Record{Key: key, Status: idempotency.StatusCompleted, ExecutionID: result.ExecutionID, ScenarioName: result.ScenarioName, ExecStatus: string(result.Status), StartedAt: result.StartedAt, FinishedAt: result.FinishedAt, Duration: result.Duration.String(), Error: result.Error}
	for _, step := range result.Steps {
		record.Steps = append(record.Steps, idempotency.StepSummary{Name: step.Name, Status: string(step.Status), Duration: step.Duration.String(), Error: step.Error})
	}
	return record
}
func rebuildExecutionResult(record idempotency.Record) ExecutionResult {
	d, _ := time.ParseDuration(record.Duration)
	r := ExecutionResult{ExecutionID: record.ExecutionID, ScenarioName: record.ScenarioName, Status: ExecutionStatus(record.ExecStatus), StartedAt: record.StartedAt, FinishedAt: record.FinishedAt, Duration: d, Error: record.Error, Passed: record.ExecStatus == string(ExecutionPassed)}
	for _, s := range record.Steps {
		sd, _ := time.ParseDuration(s.Duration)
		r.Steps = append(r.Steps, StepResult{Name: s.Name, Status: StepStatus(s.Status), Duration: sd, Error: s.Error})
	}
	return r
}

func executeSingleStep(ctx ExecutionContext, step scenario.Step, client *http.Client, requestCtx context.Context) (stepResult StepResult) {
	startedAt := time.Now()
	var tracedReq *http.Request
	var tracedReqBody string
	var tracedRespHeaders http.Header
	var tracedRespBody string
	defer func() {
		stepResult.StartedAt = startedAt
		stepResult.FinishedAt = time.Now()
		stepResult.Duration = stepResult.FinishedAt.Sub(startedAt)
		attempt := AttemptResult{Number: 1, StartedAt: startedAt, FinishedAt: stepResult.FinishedAt, Duration: stepResult.Duration, StatusCode: stepResult.StatusCode, Error: stepResult.Error}
		if stepResult.Status == StepPassed {
			attempt.Status = AttemptPassed
		} else {
			attempt.Status = AttemptFailed
		}
		if ctx.TraceEnabled && tracedReq != nil {
			attempt.HTTPMethod = tracedReq.Method
			attempt.HTTPURL = tracedReq.URL.String()
			attempt.RequestHeaders = flattenHeader(tracedReq.Header)
			attempt.RequestBody = truncate(tracedReqBody, 2048)
			attempt.ResponseHeaders = flattenHeader(tracedRespHeaders)
			attempt.ResponseBody = truncate(tracedRespBody, 2048)
		}
		stepResult.Attempts = []AttemptResult{attempt}
	}()
	stepResult = StepResult{
		Name:       step.Name,
		Status:     StepPassed,
		Assertions: make([]AssertionResult, 0),
	}

	// 1. Find the operation in the OpenAPI spec
	// The operation name refers to the operationId in the OpenAPI spec
	foundOp, foundPath, foundMethod, _ := openapi.FindOperation(ctx.Operations, step.Operation)

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
			// Template pre-pass, then variable substitution
			substituted, err := resolve(val, ctx.VariableStore, step.Name)
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
				substituted, err := resolve(v, ctx.VariableStore, step.Name)
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
				substituted, err := resolve(v, ctx.VariableStore, step.Name)
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
			substituted, err := resolve(step.Request.Body, ctx.VariableStore, step.Name)
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
	req = req.WithContext(requestCtx)
	tracedReq = req
	tracedReqBody = string(reqSpec.Body)

	// 3. Execute the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("HTTP request failed: %v", err)
		return stepResult
	}
	defer resp.Body.Close()

	stepResult.StatusCode = resp.StatusCode
	tracedRespHeaders = resp.Header

	// 4. Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		stepResult.Status = StepFailed
		stepResult.Error = fmt.Sprintf("failed to read response body: %v", err)
		return stepResult
	}
	tracedRespBody = string(bodyBytes)

	// 4b. Validate the response against the OpenAPI response schema (opt-in).
	if ctx.SchemaMode != "" && ctx.SchemaMode != schema.Off {
		sr := schema.Validate(*foundOp, resp.StatusCode, resp.Header.Get("Content-Type"), bodyBytes)
		stepResult.SchemaViolations = sr.Violations
		stepResult.UndeclaredFields = sr.UndeclaredFields
		if ctx.SchemaMode == schema.Strict && len(sr.Violations) > 0 {
			stepResult.Status = StepFailed
			if stepResult.Error == "" {
				stepResult.Error = fmt.Sprintf("schema violation: %s", sr.Violations[0].Message)
			}
		}
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
