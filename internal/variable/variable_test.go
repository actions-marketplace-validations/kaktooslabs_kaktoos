package variable

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// --- Unit tests: Extract + coerceToString ---

func TestCoerceToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"whole float", float64(123), "123"},
		{"fractional float", 123.45, "123.45"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, "null"},
		{"object", map[string]interface{}{"key": "value"}, `{"key":"value"}`},
		{"array", []interface{}{float64(1), float64(2)}, `[1,2]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coerceToString(tt.input); got != tt.expected {
				t.Errorf("coerceToString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractScalars(t *testing.T) {
	body := []byte(`{"customer":{"id":101,"email":"test@example.com"},"active":true,"note":null,"score":9.5}`)
	s := NewStore()
	err := Extract(body, map[string]string{
		"id":     "$.customer.id",
		"email":  "$.customer.email",
		"active": "$.active",
		"note":   "$.note",
		"score":  "$.score",
	}, s, "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"id": "101", "email": "test@example.com", "active": "true", "note": "null", "score": "9.5"}
	for k, v := range want {
		if got, _ := s.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestExtractObjectAndArray(t *testing.T) {
	body := []byte(`{"items":[{"sku":"A1"},{"sku":"B2"}],"meta":{"page":1}}`)
	s := NewStore()
	if err := Extract(body, map[string]string{"items": "$.items", "meta": "$.meta"}, s, "step-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := s.Get("items"); got != `[{"sku":"A1"},{"sku":"B2"}]` {
		t.Errorf("items = %q", got)
	}
	if got, _ := s.Get("meta"); got != `{"page":1}` {
		t.Errorf("meta = %q", got)
	}
}

func TestExtractOverwrites(t *testing.T) {
	s := NewStore()
	s.Set("id", "old")
	if err := Extract([]byte(`{"id":42}`), map[string]string{"id": "$.id"}, s, "step-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := s.Get("id"); got != "42" {
		t.Errorf("id = %q, want 42", got)
	}
}

func TestExtractNonJSONBody(t *testing.T) {
	err := Extract([]byte("<xml/>"), map[string]string{"x": "$.x"}, NewStore(), "step-1")
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("want 'not valid JSON' error, got %v", err)
	}
}

func TestExtractNoMatch(t *testing.T) {
	err := Extract([]byte(`{"a":1}`), map[string]string{"missing": "$.nope"}, NewStore(), "step-1")
	if err == nil {
		t.Fatal("want error for no match, got nil")
	}
	for _, want := range []string{"step-1", "missing", "$.nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestSeedCopiesEntries(t *testing.T) {
	s := NewStore()
	s.Seed(map[string]string{"a": "1", "b": "2"})
	if got, _ := s.Get("a"); got != "1" {
		t.Errorf("a = %q", got)
	}
	if got, _ := s.Get("b"); got != "2" {
		t.Errorf("b = %q", got)
	}
}

// --- Property tests (pgregory.net/rapid) ---

var identGen = rapid.StringMatching(`[a-zA-Z_][a-zA-Z0-9_]{0,15}`)

// Property 9: for any store contents, every {{key}} in a template resolves to its value.
func TestPropSubstitutionCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := identGen.Draw(t, "key")
		val := rapid.StringMatching(`[a-zA-Z0-9 ]{0,20}`).Draw(t, "val")

		s := NewStore()
		s.Set(key, val)

		got, err := Substitute("x {{"+key+"}} y", s, "step")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "x "+val+" y" {
			t.Errorf("got %q, want %q", got, "x "+val+" y")
		}
	})
}

// Property: a missing variable always errors, naming the variable and step.
func TestPropMissingVariableErrors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := identGen.Draw(t, "key")
		s := NewStore()
		_, err := Substitute("{{"+key+"}}", s, "step-x")
		if err == nil {
			t.Fatalf("expected error for missing variable %s", key)
		}
		if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "step-x") {
			t.Errorf("error %q must name %q and step-x", err.Error(), key)
		}
	})
}

// Property: extracted values overwrite env-seeded values (extraction precedence).
func TestPropExtractedPrecedence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := identGen.Draw(t, "key")
		seeded := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "seeded")
		extracted := rapid.StringMatching(`[A-Z]{1,10}`).Draw(t, "extracted")

		s := NewStore()
		s.Seed(map[string]string{key: seeded})

		body := []byte(fmt.Sprintf(`{"v":%q}`, extracted))
		if err := Extract(body, map[string]string{key: "$.v"}, s, "step"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, _ := s.Get(key); got != extracted {
			t.Errorf("got %q, want extracted value %q", got, extracted)
		}
	})
}

// Property: extraction of a string scalar round-trips exactly.
func TestPropJSONPathExtractionCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := identGen.Draw(t, "key")
		val := rapid.StringMatching(`[a-zA-Z0-9]{0,20}`).Draw(t, "val")

		body := []byte(fmt.Sprintf(`{%q:%q}`, key, val))
		s := NewStore()
		if err := Extract(body, map[string]string{"out": "$." + key}, s, "step"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, _ := s.Get("out"); got != val {
			t.Errorf("got %q, want %q", got, val)
		}
	})
}

// Property: a no-match JSONPath always errors and leaves the store untouched.
func TestPropNoMatchHaltsExtraction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := identGen.Draw(t, "key")
		s := NewStore()
		err := Extract([]byte(`{"other":"data"}`), map[string]string{key: "$.missing_" + key}, s, "step")
		if err == nil {
			t.Fatalf("expected error for no-match path")
		}
		if _, ok := s.Get(key); ok {
			t.Errorf("store must not contain %q after failed extraction", key)
		}
	})
}
