package engine_test

import (
	"errors"
	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"github.com/kaktooslabs/kaktoos/internal/idempotency"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
	"net/http"
	"sync/atomic"
	"testing"
)

type failingTransport struct{ calls atomic.Int32 }

func (f *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return nil, errors.New("connection refused")
}
func reliableScenario() scenario.Scenario {
	return scenario.Scenario{Name: "retry", Steps: []scenario.Step{{Name: "request", Operation: "op", Retry: &scenario.RetryPolicy{Strategy: "fixed_delay", MaxAttempts: 3, InitialDelay: "10ms", RetryOn: scenario.RetryConditions{NetworkErrors: true}}}}}
}
func reliableOps() openapi.OperationMap {
	return openapi.OperationMap{"/": openapi.OperationMapInner{"GET": openapi.Operation{Name: "op"}}}
}
func TestNetworkErrorRetriesAndRecordsAttempts(t *testing.T) {
	tr := &failingTransport{}
	r := engine.RunScenario(engine.NewContext(config.Environment{BaseURL: "http://example.test"}, reliableOps()), reliableScenario(), &http.Client{Transport: tr})
	if tr.calls.Load() != 3 || len(r.Steps[0].Attempts) != 3 || r.Status != engine.ExecutionFailed {
		t.Fatalf("calls=%d result=%+v", tr.calls.Load(), r)
	}
}
func TestIdempotencyReturnsCachedResult(t *testing.T) {
	tr := &failingTransport{}
	ctx := engine.NewContext(config.Environment{BaseURL: "http://example.test", Variables: map[string]string{"key": "x"}}, reliableOps())
	ctx.Idempotency = idempotency.NewMemoryStore()
	s := reliableScenario()
	s.IdempotencyKey = "{{key}}"
	engine.RunScenario(ctx, s, &http.Client{Transport: tr})
	n := tr.calls.Load()
	ctx2 := engine.NewContext(config.Environment{BaseURL: "http://example.test", Variables: map[string]string{"key": "x"}}, reliableOps())
	ctx2.Idempotency = ctx.Idempotency
	r := engine.RunScenario(ctx2, s, &http.Client{Transport: tr})
	if tr.calls.Load() != n || r.ExecutionID == "" {
		t.Fatalf("cache failed: %d %d", n, tr.calls.Load())
	}
}
