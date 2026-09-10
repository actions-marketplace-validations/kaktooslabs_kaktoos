package engine

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kaktooslabs/kaktoos/internal/config"
	"github.com/kaktooslabs/kaktoos/internal/openapi"
	"github.com/kaktooslabs/kaktoos/internal/ratelimit"
	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

type scriptedTransport struct {
	statuses []int
	calls    int
}

func (s *scriptedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	code := s.statuses[s.calls]
	s.calls++
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
}

type waitingTransport struct{}

func (waitingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	<-r.Context().Done()
	return nil, r.Context().Err()
}
func engineOps() openapi.OperationMap { return openapi.OperationMap{"/": {"GET": {Name: "op"}}} }
func retryStep(policy *scenario.RetryPolicy) scenario.Step {
	return scenario.Step{Name: "step", Operation: "op", Retry: policy}
}

func TestRetryableStatusRetries(t *testing.T) {
	tr := &scriptedTransport{statuses: []int{503, 503, 200}}
	p := &scenario.RetryPolicy{Strategy: "fixed_delay", MaxAttempts: 3, InitialDelay: "10ms", RetryOn: scenario.RetryConditions{StatusCodes: []int{503}}}
	r := RunScenario(NewContext(config.Environment{BaseURL: "http://test"}, engineOps()), scenario.Scenario{Steps: []scenario.Step{retryStep(p)}}, &http.Client{Transport: tr})
	if tr.calls != 3 || len(r.Steps[0].Attempts) != 3 {
		t.Fatalf("calls=%d attempts=%d", tr.calls, len(r.Steps[0].Attempts))
	}
}
func TestNonRetryableStatusDoesNotRetry(t *testing.T) {
	tr := &scriptedTransport{statuses: []int{400}}
	p := &scenario.RetryPolicy{Strategy: "fixed_delay", MaxAttempts: 3, InitialDelay: "10ms", RetryOn: scenario.RetryConditions{StatusCodes: []int{503}}}
	r := RunScenario(NewContext(config.Environment{BaseURL: "http://test"}, engineOps()), scenario.Scenario{Steps: []scenario.Step{retryStep(p)}}, &http.Client{Transport: tr})
	if tr.calls != 1 || len(r.Steps[0].Attempts) != 1 {
		t.Fatal("unexpected retry")
	}
}
func TestRetryDelayStrategiesAndCap(t *testing.T) {
	fixed := &scenario.RetryPolicy{Strategy: "fixed_delay", InitialDelay: "10ms"}
	if retryDelay(fixed, 3) != 10*time.Millisecond {
		t.Fatal("fixed")
	}
	linear := &scenario.RetryPolicy{Strategy: "linear_backoff", InitialDelay: "10ms"}
	if retryDelay(linear, 3) != 30*time.Millisecond {
		t.Fatal("linear")
	}
	exp := &scenario.RetryPolicy{Strategy: "exponential_backoff", InitialDelay: "10ms", BackoffMultiplier: 2, MaxDelay: "30ms"}
	if retryDelay(exp, 3) != 30*time.Millisecond {
		t.Fatal("exp cap")
	}
}
func TestStepTimeoutCancelsRequest(t *testing.T) {
	s := scenario.Scenario{Steps: []scenario.Step{{Name: "slow", Operation: "op", Timeout: "10ms"}}}
	r := RunScenario(NewContext(config.Environment{BaseURL: "http://test"}, engineOps()), s, &http.Client{Transport: waitingTransport{}})
	if r.Steps[0].Status != StepTimedOut || r.Status != ExecutionTimedOut {
		t.Fatalf("%+v", r)
	}
}
func TestWorkflowTimeoutCancelsRequest(t *testing.T) {
	s := scenario.Scenario{WorkflowTimeout: "10ms", Steps: []scenario.Step{{Name: "slow", Operation: "op"}, {Name: "skipped", Operation: "op"}}}
	r := RunScenario(NewContext(config.Environment{BaseURL: "http://test"}, engineOps()), s, &http.Client{Transport: waitingTransport{}})
	if r.Status != ExecutionTimedOut || r.Steps[0].Status != StepTimedOut || r.Steps[1].Status != StepSkipped {
		t.Fatalf("%+v", r)
	}
}
func TestAssertionFailureDoesNotRetry(t *testing.T) {
	tr := &scriptedTransport{statuses: []int{200}}
	expected := 201
	p := &scenario.RetryPolicy{Strategy: "fixed_delay", MaxAttempts: 3, InitialDelay: "10ms", RetryOn: scenario.RetryConditions{NetworkErrors: true}}
	s := retryStep(p)
	s.Assert = &scenario.AssertSpec{Status: &expected}
	r := RunScenario(NewContext(config.Environment{BaseURL: "http://test"}, engineOps()), scenario.Scenario{Steps: []scenario.Step{s}}, &http.Client{Transport: tr})
	if tr.calls != 1 || len(r.Steps[0].Attempts) != 1 {
		t.Fatal("assertion failure retried")
	}
}
func TestRateLimitWaitsPerAttempt(t *testing.T) {
	rateValue := 10
	l := ratelimit.NewLimiter(map[string]config.RateLimitEntry{"op": {RequestsPerSecond: &rateValue}})
	tr := &scriptedTransport{statuses: []int{503, 503, 200}}
	p := &scenario.RetryPolicy{Strategy: "fixed_delay", MaxAttempts: 3, InitialDelay: "10ms", RetryOn: scenario.RetryConditions{StatusCodes: []int{503}}}
	ctx := NewContext(config.Environment{BaseURL: "http://test"}, engineOps())
	ctx.RateLimiter = l
	started := time.Now()
	RunScenario(ctx, scenario.Scenario{Steps: []scenario.Step{retryStep(p)}}, &http.Client{Transport: tr})
	if elapsed := time.Since(started); elapsed < 180*time.Millisecond {
		t.Fatalf("rate limiter was bypassed: %s", elapsed)
	}
}
func TestLimiterCancellation(t *testing.T) {
	rateValue := 1
	l := ratelimit.NewLimiter(map[string]config.RateLimitEntry{"op": {RequestsPerSecond: &rateValue}})
	if err := l.Wait(context.Background(), "op"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx, "op"); err == nil {
		t.Fatal("wanted cancellation")
	}
}
