// Package verification owns the vocabulary Kaktoos uses to say *why* a step
// failed. The engine classifies once; every output (text, JSON, JUnit, MCP,
// GitHub Action) reads the result instead of re-deriving it from error strings.
package verification

import (
	"context"
	"errors"
	"strings"
)

// Category is a deterministic, machine-comparable failure class.
type Category string

const (
	// CategoryContractMismatch: the response violated the OpenAPI contract
	// (missing required field, wrong type, undeclared status, wrong content type).
	CategoryContractMismatch Category = "contract_mismatch"
	// CategoryAssertionFailed: a scenario assertion or condition was false.
	// A read-back that finds state missing after a successful write lands here.
	CategoryAssertionFailed Category = "assertion_failed"
	// CategoryAuthFailure: 401 or 403.
	CategoryAuthFailure Category = "auth_failure"
	// CategoryRateLimited: 429.
	CategoryRateLimited Category = "rate_limited"
	// CategoryServerError: any 5xx.
	CategoryServerError Category = "server_error"
	// CategoryTransportFailure: no HTTP response at all (refused, DNS, reset).
	CategoryTransportFailure Category = "transport_failure"
	// CategoryTimeout: the step or workflow deadline expired.
	CategoryTimeout Category = "timeout"
	// CategoryConfigError: the scenario itself is wrong — unknown operation,
	// unresolved variable, bad extract path, bad condition syntax.
	CategoryConfigError Category = "config_error"
)

// Input is everything Classify needs. It is a struct rather than positional
// arguments so call sites read unambiguously.
type Input struct {
	StatusCode          int
	HasSchemaViolations bool
	HasFailedAssertion  bool
	// TransportErr is the error from client.Do, if any.
	TransportErr error
	// TimedOut is set when a context deadline fired.
	TimedOut bool
	// ConfigErr is set for pre-request or extract failures caused by the
	// scenario (not the API).
	ConfigErr bool
}

// Classify maps a failed step to exactly one Category. Precedence, highest
// first: timeout, transport, config, status-derived (auth/rate-limit/5xx),
// contract, assertion. Status wins over assertion because a 401 that fails
// `assert: {status: 200}` is an auth problem, not a wrong assertion.
func Classify(in Input) Category {
	switch {
	case in.TimedOut || isDeadline(in.TransportErr):
		return CategoryTimeout
	case in.TransportErr != nil:
		return CategoryTransportFailure
	case in.ConfigErr:
		return CategoryConfigError
	case in.StatusCode == 401 || in.StatusCode == 403:
		return CategoryAuthFailure
	case in.StatusCode == 429:
		return CategoryRateLimited
	case in.StatusCode >= 500 && in.StatusCode <= 599:
		return CategoryServerError
	case in.HasSchemaViolations:
		return CategoryContractMismatch
	default:
		return CategoryAssertionFailed
	}
}

func isDeadline(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var t interface{ Timeout() bool }
	if errors.As(err, &t) && t.Timeout() {
		return true
	}
	// net/http wraps client timeouts without a typed error in some paths.
	return strings.Contains(err.Error(), "Client.Timeout exceeded")
}

// RedactHeaders returns a copy of h with credential-bearing headers masked.
// Header name matching is case-insensitive. Never mutates its input.
func RedactHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		switch strings.ToLower(k) {
		case "authorization", "proxy-authorization", "x-api-key", "x-auth-token", "cookie", "set-cookie":
			out[k] = "***"
		default:
			out[k] = v
		}
	}
	return out
}
