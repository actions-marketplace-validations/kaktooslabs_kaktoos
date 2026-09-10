package tracer

import (
	"bytes"
	"github.com/kaktooslabs/kaktoos/internal/engine"
	"strings"
	"testing"
)

func TestTracerDisabled(t *testing.T) {
	var b bytes.Buffer
	if e := New(false, FormatJSON, false).Write(engine.ExecutionResult{}, &b); e != nil || b.Len() != 0 {
		t.Fatal("disabled tracer wrote output")
	}
}
func TestTracerMasksAndTruncates(t *testing.T) {
	r := engine.ExecutionResult{Steps: []engine.StepResult{{Attempts: []engine.AttemptResult{{RequestHeaders: map[string]string{"Authorization": "secret"}, RequestBody: strings.Repeat("x", 3000)}}}}}
	var b bytes.Buffer
	if e := New(true, FormatJSON, false).Write(r, &b); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(b.String(), "***") || strings.Contains(b.String(), "secret") {
		t.Fatal(b.String())
	}
	if strings.Count(b.String(), "x") > 2050 {
		t.Fatal("body not truncated")
	}
}
