package assertion

// Result holds the outcome of a single assertion.
type Result struct {
	Type     string      // "status", "equals", "exists", "not_exists"
	Path     string      // JSONPath for body assertions; empty for status
	Expected interface{} // expected value (status code or equals value)
	Actual   interface{} // observed value
	Passed   bool
	Error    string // evaluation error (non-JSON body, bad expression, ...)
}

// BodyAssert is one JSONPath assertion against the response body.
// Exactly one of Equals / Exists / NotExists is expected to be set.
type BodyAssert struct {
	Path      string
	Equals    interface{}
	Exists    bool
	NotExists bool
}

// Spec is the assert block of a step.
// ponytail: mirrors the design's scenario.AssertSpec; scenario/types.go currently
// diverges from the design — reconcile there and swap this for scenario.AssertSpec.
type Spec struct {
	Status *int
	Body   []BodyAssert
}
