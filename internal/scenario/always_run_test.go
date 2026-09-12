package scenario

import (
	"strings"
	"testing"
)

func TestLoad_AlwaysRunRoundTrips(t *testing.T) {
	s, err := Load([]byte(`
name: t
steps:
  - name: create
    operation: createOrder
  - name: cleanup
    operation: deleteOrder
    always_run: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Steps[0].AlwaysRun || !s.Steps[1].AlwaysRun {
		t.Fatalf("always_run parsed wrong: %+v", s.Steps)
	}
}

func TestLoad_AlwaysRunRejectedOnConditionStep(t *testing.T) {
	_, err := Load([]byte(`
name: t
steps:
  - name: gate
    condition: $x exists
    always_run: true
`))
	if err == nil || !strings.Contains(err.Error(), "always_run is only valid on operation steps") {
		t.Fatalf("expected always_run-on-condition error, got %v", err)
	}
}
