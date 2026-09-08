package assertion

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func resp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func intp(i int) *int { return &i }

func TestStatusAssertion(t *testing.T) {
	r := Evaluate(resp(200, ""), Spec{Status: intp(200)})
	if len(r) != 1 || !r[0].Passed {
		t.Fatalf("want pass, got %+v", r)
	}
	r = Evaluate(resp(500, ""), Spec{Status: intp(200)})
	if r[0].Passed || r[0].Expected != 200 || r[0].Actual != 500 {
		t.Fatalf("want fail with expected=200 actual=500, got %+v", r[0])
	}
}

func TestBodyEquals(t *testing.T) {
	body := `{"status":"ok","count":3,"tags":["a","b"]}`

	r := Evaluate(resp(200, body), Spec{Body: []BodyAssert{{Path: "$.status", Equals: "ok"}}})
	if !r[0].Passed {
		t.Errorf("string equals should pass: %+v", r[0])
	}

	r = Evaluate(resp(200, body), Spec{Body: []BodyAssert{{Path: "$.status", Equals: "nope"}}})
	if r[0].Passed || r[0].Expected != "nope" || r[0].Actual != "ok" {
		t.Errorf("mismatch should fail with expected/actual recorded: %+v", r[0])
	}

	// YAML int vs JSON float64
	r = Evaluate(resp(200, body), Spec{Body: []BodyAssert{{Path: "$.count", Equals: 3}}})
	if !r[0].Passed {
		t.Errorf("numeric normalisation should pass: %+v", r[0])
	}

	r = Evaluate(resp(200, body), Spec{Body: []BodyAssert{{Path: "$.tags", Equals: []interface{}{"a", "b"}}}})
	if !r[0].Passed {
		t.Errorf("array equals should pass: %+v", r[0])
	}
}

func TestBodyExistsNotExists(t *testing.T) {
	body := `{"id":1}`
	cases := []struct {
		ba   BodyAssert
		want bool
	}{
		{BodyAssert{Path: "$.id", Exists: true}, true},
		{BodyAssert{Path: "$.missing", Exists: true}, false},
		{BodyAssert{Path: "$.missing", NotExists: true}, true},
		{BodyAssert{Path: "$.id", NotExists: true}, false},
	}
	for _, c := range cases {
		r := Evaluate(resp(200, body), Spec{Body: []BodyAssert{c.ba}})[0]
		if r.Passed != c.want {
			t.Errorf("%+v: passed=%v want %v (%s)", c.ba, r.Passed, c.want, r.Error)
		}
		if c.ba.NotExists && !c.want && r.Actual == nil {
			t.Errorf("not_exists failure must record matched value: %+v", r)
		}
	}
}

func TestNonJSONBodyFailsAllBodyAssertions(t *testing.T) {
	r := Evaluate(resp(200, "<xml/>"), Spec{
		Status: intp(200),
		Body:   []BodyAssert{{Path: "$.a", Exists: true}, {Path: "$.b", Equals: 1}},
	})
	if len(r) != 3 || !r[0].Passed {
		t.Fatalf("status should still pass, got %+v", r)
	}
	for _, br := range r[1:] {
		if br.Passed || !strings.Contains(br.Error, "not valid JSON") {
			t.Errorf("body assertion should fail with JSON error: %+v", br)
		}
	}
}

func TestInvalidJSONPath(t *testing.T) {
	r := Evaluate(resp(200, `{"a":1}`), Spec{Body: []BodyAssert{{Path: "$[", Equals: 1}}})[0]
	if r.Passed || r.Error == "" {
		t.Fatalf("invalid expression should fail with error: %+v", r)
	}
}

func TestEmptySpecReturnsEmptyNonNil(t *testing.T) {
	r := Evaluate(resp(200, ""), Spec{})
	if r == nil || len(r) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v", r)
	}
}

// Property 17: status assertion passes iff expected == actual.
func TestPropStatusEquality(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e := rapid.IntRange(100, 599).Draw(t, "expected")
		a := rapid.IntRange(100, 599).Draw(t, "actual")
		r := Evaluate(resp(a, ""), Spec{Status: intp(e)})[0]
		if r.Passed != (e == a) {
			t.Fatalf("e=%d a=%d passed=%v", e, a, r.Passed)
		}
	})
}

// Property 18: N failing assertions yield N results, all failed.
func TestPropAllAssertionsEvaluated(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(t, "n")
		body := make([]BodyAssert, n)
		for i := range body {
			body[i] = BodyAssert{Path: "$.missing", Exists: true}
		}
		r := Evaluate(resp(200, `{"a":1}`), Spec{Body: body})
		if len(r) != n {
			t.Fatalf("len=%d want %d", len(r), n)
		}
		for _, x := range r {
			if x.Passed {
				t.Fatalf("expected all failed, got %+v", x)
			}
		}
	})
}
