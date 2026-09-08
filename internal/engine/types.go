package engine

import (
	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/variable"
)

// StepStatus is the status of a single step during scenario execution.
type StepStatus string

// Constants for step status
const (
	StepPassed  StepStatus = "PASSED"
	StepFailed  StepStatus = "FAILED"
	StepSkipped StepStatus = "SKIPPED"
)

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
	Name       string
	Status     StepStatus
	StatusCode int
	Assertions []AssertionResult // Results from the assertion engine
	Error      string            // General error for the step (e.g., networking, variable substitution)
}

// ExecutionResult holds the summary of a single scenario run.
type ExecutionResult struct {
	ScenarioName string
	Passed       bool
	Steps        []StepResult
}

// ExecutionContext holds the state required to run a scenario.
type ExecutionContext struct {
	Environment   config.Environment
	Operations    openapi.OperationMap
	VariableStore *variable.Store // Pointer to the store to manage state
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
