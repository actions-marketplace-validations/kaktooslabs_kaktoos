package engine_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

func TestPhase2WorkflowTemplateExtractionConditionAndTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			_, _ = fmt.Fprint(w, `{"name":"alice","active":"yes"}`)
			return
		}
		if r.URL.Path != "/users/ALICE" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization was not substituted")
		}
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	status := 200
	ops := openapi.OperationMap{"/seed": {"GET": {Name: "seed"}}, "/users/{name}": {"GET": {Name: "user"}}}
	s := scenario.Scenario{Name: "workflow", Steps: []scenario.Step{
		{Name: "seed", Operation: "seed", Extract: map[string]string{"name": "$.name", "active": "$.active"}},
		{Name: "condition", Condition: "$.active equals 'yes'"},
		{Name: "user", Operation: "user", Request: &scenario.RequestSpec{Path: map[string]string{"name": "{{toUpper(name)}}"}, Headers: map[string]string{"Authorization": "Bearer {{secret}}"}}, Assert: &scenario.AssertSpec{Status: &status}},
	}}
	ctx := engine.NewContext(config.Environment{BaseURL: server.URL, Variables: map[string]string{"secret": "secret"}}, ops)
	ctx.TraceEnabled = true
	r := engine.RunScenario(ctx, s, http.DefaultClient)
	if r.Status != engine.ExecutionPassed {
		t.Fatalf("result: %+v", r)
	}
	if len(r.Steps[2].Attempts) != 1 || r.Steps[2].Attempts[0].HTTPURL == "" {
		t.Fatalf("missing trace attempt metadata: %+v", r.Steps[2])
	}
}
