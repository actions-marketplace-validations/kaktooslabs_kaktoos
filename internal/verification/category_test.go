package verification

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want Category
	}{
		{"timeout flag wins over everything", Input{TimedOut: true, StatusCode: 200, HasFailedAssertion: true}, CategoryTimeout},
		{"deadline exceeded error", Input{TransportErr: context.DeadlineExceeded}, CategoryTimeout},
		{"transport failure", Input{TransportErr: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}, CategoryTransportFailure},
		{"config error beats status", Input{ConfigErr: true, StatusCode: 401}, CategoryConfigError},
		{"401 is auth, not assertion", Input{StatusCode: 401, HasFailedAssertion: true}, CategoryAuthFailure},
		{"403 is auth", Input{StatusCode: 403, HasFailedAssertion: true}, CategoryAuthFailure},
		{"429 is rate limited", Input{StatusCode: 429, HasFailedAssertion: true}, CategoryRateLimited},
		{"500 is server error", Input{StatusCode: 500, HasFailedAssertion: true}, CategoryServerError},
		{"599 is server error", Input{StatusCode: 599}, CategoryServerError},
		{"schema violation on 200 is contract mismatch", Input{StatusCode: 200, HasSchemaViolations: true}, CategoryContractMismatch},
		{"plain failed assertion on 200", Input{StatusCode: 200, HasFailedAssertion: true}, CategoryAssertionFailed},
		{"state not persisted: 200 write, 404 read-back", Input{StatusCode: 404, HasFailedAssertion: true}, CategoryAssertionFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.in); got != c.want {
				t.Errorf("Classify(%+v) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestRedactHeaders(t *testing.T) {
	in := map[string]string{
		"Authorization": "Bearer secret",
		"Cookie":        "session=abc",
		"X-Api-Key":     "k1",
		"Content-Type":  "application/json",
	}
	out := RedactHeaders(in)
	if out["Authorization"] != "***" || out["Cookie"] != "***" || out["X-Api-Key"] != "***" {
		t.Fatalf("sensitive headers not redacted: %+v", out)
	}
	if out["Content-Type"] != "application/json" {
		t.Fatalf("non-sensitive header altered: %+v", out)
	}
	// original untouched
	if in["Authorization"] != "Bearer secret" {
		t.Fatalf("RedactHeaders mutated its input")
	}
}

func TestRedactHeadersNil(t *testing.T) {
	if RedactHeaders(nil) != nil {
		t.Fatalf("expected nil in, nil out")
	}
}
