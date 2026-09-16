package discovery

import (
	"strings"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	cfg, err := LoadConfigBytes([]byte(`
services:
  - name: payment-service
    paths: ["payment-service/**"]
    openapi: payment-service/openapi.yml
    scenarios: payment-service/scenarios
    depends_on: [ledger-service]
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	s := cfg.Services[0]
	if s.Name != "payment-service" || s.OpenAPI != "payment-service/openapi.yml" || len(s.DependsOn) != 1 {
		t.Fatalf("parsed wrong: %+v", s)
	}
}

func TestLoadConfigRejections(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"unknown top-level field": {"services:\n  - name: a\n    paths: [\"a/**\"]\nregistry: x\n", "unknown field"},
		"unknown service field":   {"services:\n  - name: a\n    paths: [\"a/**\"]\n    openpai: x\n", "unknown field"},
		"missing name":            {"services:\n  - paths: [\"a/**\"]\n", "name is required"},
		"missing paths":           {"services:\n  - name: a\n", "path glob is required"},
		"duplicate name":          {"services:\n  - name: a\n    paths: [\"a/**\"]\n  - name: a\n    paths: [\"b/**\"]\n", "duplicate service name"},
		"no services":             {"services: []\n", "at least one service"},
		"not yaml":                {"services: [oops\n", "not valid YAML"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfigBytes([]byte(tc.yaml))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestFindConfig(t *testing.T) {
	if got := FindConfig(fixture); got == "" {
		t.Fatal("fixture repo should have a manifest")
	}
	if got := FindConfig(t.TempDir()); got != "" {
		t.Fatalf("empty dir should have no manifest, got %q", got)
	}
}
