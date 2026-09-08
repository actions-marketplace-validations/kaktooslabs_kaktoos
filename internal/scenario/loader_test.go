package scenario_test

import (
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"github.com/stretchr/testify/assert"
)

func TestLoad_UnknownFieldRejection(t *testing.T) {
	// Scenario YAML with an unknown field at the root level
	unknownFieldYAML := []byte(`
name: BadScenario
unknownField: shouldFail
steps:
  - name: StepOne
    operation: someOperation
`)

	// Scenario YAML with an unknown field in a step
	stepUnknownFieldYAML := []byte(`
name: BadScenario
steps:
  - name: StepOne
    operation: someOperation
    unknownStepField: true
`)

	t.Run("Root unknown field", func(t *testing.T) {
		_, err := scenario.Load(unknownFieldYAML)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown field in Scenario: unknownField")
	})

	t.Run("Step unknown field", func(t *testing.T) {
		_, err := scenario.Load(stepUnknownFieldYAML)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown field in Scenario step 0: unknownStepField")
	})
}

func TestLoad_IntegrationSuccess(t *testing.T) {
	validYAML := []byte(`
name: PaymentSuccess
steps:
  - name: Check status code
    operation: getStatus
    request:
      operationName: getStatus
      headers:
        Accept: application/json
    assert:
      type: equals
      targetPath: $.success
      expectedValue: true
`)

	s, err := scenario.Load(validYAML)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "PaymentSuccess", s.Name)
	assert.Len(t, s.Steps, 1)
	assert.Equal(t, "Check status code", s.Steps[0].Name)
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Malformed YAML
	invalidYAML := []byte(`
name: BadSyntax
steps: !!invalid: {
`)

	_, err := scenario.Load(invalidYAML)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal step data into map:")
}

func TestLoad_InvalidStepStructure(t *testing.T) {
	// Step list contains a non-map item
	invalidStepListYAML := []byte(`
name: BadSteps
steps:
  - name: GoodStep
    operation: ok
  - not_a_map_item # Should fail during map parsing
`)

	_, err := scenario.Load(invalidStepListYAML)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step 1 is not a valid map structure")
}
