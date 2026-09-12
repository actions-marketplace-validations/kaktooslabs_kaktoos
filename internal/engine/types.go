package engine

import (
	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/idempotency"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/ratelimit"
	"github.com/kaktooslabs/kaktoos/internal/schema"
	"github.com/kaktooslabs/kaktoos/internal/variable"
	"github.com/kaktooslabs/kaktoos/internal/verification"
	"time"
)

// StepStatus is the status of a single step during scenario execution.
type ExecutionStatus string

const (
	ExecutionPassed    ExecutionStatus = "PASSED"
	ExecutionFailed    ExecutionStatus = "FAILED"
	ExecutionTimedOut  ExecutionStatus = "TIMED_OUT"
	ExecutionCancelled ExecutionStatus = "CANCELLED"
)

type StepStatus string

// Constants for step status
const (
	StepPassed   StepStatus = "PASSED"
	StepFailed   StepStatus = "FAILED"
	StepTimedOut StepStatus = "TIMED_OUT"
	StepSkipped  StepStatus = "SKIPPED"
)

type AttemptStatus string

const (
	AttemptPassed   AttemptStatus = "PASSED"
	AttemptFailed   AttemptStatus = "FAILED"
	AttemptTimedOut AttemptStatus = "TIMED_OUT"
)

type AttemptResult struct {
	Number                          int
	StartedAt, FinishedAt           time.Time
	Duration                        time.Duration
	Status                          AttemptStatus
	StatusCode                      int
	Error                           string
	HTTPMethod, HTTPURL             string
	RequestHeaders, ResponseHeaders map[string]string
	RequestBody, ResponseBody       string
	// FailureCategory says why this attempt failed; empty when it passed.
	FailureCategory verification.Category
}

// AssertionResult mirrors the Result struct from internal/assertion/types.go.
// It holds the result of a single assertion check within a step.
type AssertionResult struct {
	Type     string
	Path     string
	Expected interface{}
	Actual   interface{}
	Passed   bool
	Error    string
}

// StepResult encapsulates all execution data for one step.
type StepResult struct {
	Name                  string
	Status                StepStatus
	StatusCode            int
	Assertions            []AssertionResult // Results from the assertion engine
	Error                 string            // General error for the step (e.g., networking, variable substitution)
	StepType              string
	StartedAt, FinishedAt time.Time
	Duration              time.Duration
	ExtractedVars         map[string]string
	Attempts              []AttemptResult
	SchemaViolations      []schema.Violation
	UndeclaredFields      []string
	// FailureCategory is the deterministic failure class, computed once by the
	// engine and consumed verbatim by every reporter. Empty when the step passed.
	FailureCategory verification.Category
	// Evidence is the redacted request/response behind a failure; nil when the
	// step passed or no request was made.
	Evidence *verification.Evidence
}

// ExecutionResult holds the summary of a single scenario run.
type ExecutionResult struct {
	ScenarioName          string
	Passed                bool
	Steps                 []StepResult
	ExecutionID           string
	Status                ExecutionStatus
	StartedAt, FinishedAt time.Time
	Duration              time.Duration
	Error                 string
}

// ExecutionContext holds the state required to run a scenario.
type ExecutionContext struct {
	Environment   config.Environment
	Operations    openapi.OperationMap
	VariableStore *variable.Store // Pointer to the store to manage state
	RateLimiter   *ratelimit.Limiter
	Idempotency   idempotency.Store
	TraceEnabled  bool
	SchemaMode    schema.Mode
}

// NewContext creates a fresh execution context for a scenario.
func NewContext(env config.Environment, ops openapi.OperationMap) ExecutionContext {
	// Requirement 8.8: Must create a new Store for every call.
	store := variable.NewStore()
	store.Seed(env.Variables)

	return ExecutionContext{
		Environment:   env,
		Operations:    ops,
		VariableStore: store,
	}
}
