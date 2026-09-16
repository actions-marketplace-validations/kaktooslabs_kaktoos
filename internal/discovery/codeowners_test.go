package discovery

import (
	"reflect"
	"testing"
)

const ownersFile = `
# comment
*                      @platform-team
payment-service/       @payments-team @payments-oncall
payment-service/fee.go @fee-owner
*.md                   @docs-team
`

func TestCodeownersLastMatchWins(t *testing.T) {
	c := ParseCodeowners(ownersFile)
	cases := []struct {
		path string
		want []string
	}{
		{"payment-service/fee.go", []string{"@fee-owner"}},
		{"payment-service/client.go", []string{"@payments-team", "@payments-oncall"}},
		{"unrelated/thing.go", []string{"@platform-team"}},
		{"docs-site/index.md", []string{"@docs-team"}},
	}
	for _, tc := range cases {
		if got := c.Owners(tc.path); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Owners(%s) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// Ownership is contextual: a path no rule covers gets no owner, never a guess.
func TestCodeownersNoMatchReturnsNothing(t *testing.T) {
	c := ParseCodeowners("payment-service/ @payments-team\n")
	if got := c.Owners("checkout-service/checkout.go"); len(got) != 0 {
		t.Fatalf("expected no owners, got %v", got)
	}
}

func TestCodeownersMissingFileIsNotAnError(t *testing.T) {
	c, err := LoadCodeowners(t.TempDir())
	if err != nil {
		t.Fatalf("missing CODEOWNERS should not error: %v", err)
	}
	if got := c.Owners("anything.go"); len(got) != 0 {
		t.Fatalf("expected no owners, got %v", got)
	}
}

func TestCodeownersIgnoresCommentsAndBlanks(t *testing.T) {
	c := ParseCodeowners("\n# just a comment\n\nlonelypattern\n")
	if len(c.rules) != 0 {
		t.Fatalf("expected no usable rules, got %+v", c.rules)
	}
}

func TestLoadCodeownersFromFixture(t *testing.T) {
	c, err := LoadCodeowners(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Owners("payment-service/fee.go"); len(got) != 1 || got[0] != "@payments-team" {
		t.Fatalf("fixture owners: %v", got)
	}
}
