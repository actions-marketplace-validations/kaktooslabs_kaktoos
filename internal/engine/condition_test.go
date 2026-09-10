package engine_test

import (
	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"net/http"
	"testing"
)

func TestConditionStep(t *testing.T) {
	ctx := engine.NewContext(config.Environment{Variables: map[string]string{"status": "active"}}, openapi.OperationMap{})
	r := engine.RunScenario(ctx, scenario.Scenario{Name: "c", Steps: []scenario.Step{{Name: "c", Condition: "$.status equals 'active'"}}}, http.DefaultClient)
	if r.Status != engine.ExecutionPassed || r.Steps[0].StepType != "condition" {
		t.Fatalf("%+v", r)
	}
}
