package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const inlineTestYAML = `
name: inline test
steps:
  - name: step1
    operation: getUser
`

func TestLoadScenarios_InlineMatchesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.yml")
	if err := os.WriteFile(p, []byte(inlineTestYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	fromFile, err := loadScenarios([]string{p}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fromInline, err := loadScenarios(nil, []string{inlineTestYAML})
	if err != nil {
		t.Fatal(err)
	}
	if len(fromFile) != 1 || len(fromInline) != 1 {
		t.Fatalf("expected one scenario each, got %d and %d", len(fromFile), len(fromInline))
	}
	if fromFile[0].Name != fromInline[0].Name || fromFile[0].Steps[0].Operation != fromInline[0].Steps[0].Operation {
		t.Fatalf("inline and file scenarios diverged: %+v vs %+v", fromFile[0], fromInline[0])
	}
}

func TestLoadScenarios_InlineDoesNoFileIO(t *testing.T) {
	// No files exist at all; this must still succeed since it's inline-only.
	scenarios, err := loadScenarios(nil, []string{inlineTestYAML})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected one scenario, got %d", len(scenarios))
	}
}

func TestLoadScenarios_BadInlineYAMLSurfacesError(t *testing.T) {
	_, err := loadScenarios(nil, []string{"not: valid: yaml: at all: ["})
	if err == nil {
		t.Fatal("expected an error for malformed inline YAML")
	}
	if !strings.Contains(err.Error(), "inline scenario") {
		t.Fatalf("expected error to identify the inline scenario, got %v", err)
	}
}
