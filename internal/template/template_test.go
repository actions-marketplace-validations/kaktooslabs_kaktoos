package template

import (
	"github.com/kaktooslabs/kaktoos/internal/variable"
	"testing"
)

func TestResolve(t *testing.T) {
	s := variable.NewStore()
	s.Set("name", "alice")
	got, e := Resolve("{{toUpper(name)}} {{name}}", s, "x")
	if e != nil || got != "ALICE {{name}}" {
		t.Fatalf("%q %v", got, e)
	}
}
