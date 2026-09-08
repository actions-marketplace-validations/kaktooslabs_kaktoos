package variable

import (
	"reflect"
	"testing"
)

// Mock implementation of Store methods used in the test file for independent testing.
// In a real scenario, we'd use the actual Store methods.

// TestSubstituteSuccess tests basic and multiple successful substitutions.
func TestSubstituteSuccess(t *testing.T) {
	store := NewStore()
	store.Set("user", "Alice")
	store.Set("repo", "kaktoos")
	store.Set("env", "staging")

	input := "Testing deployment for {{user}} to {{repo}} in {{env}}."
	expected := "Testing deployment for Alice to kaktoos in staging."

	result, err := Substitute(input, store, "test-step")
	if err != nil {
		t.Fatalf("Substitute returned an unexpected error: %v", err)
	}
	if result != expected {
		t.Errorf("Substitute failed. Got: %s, Want: %s", result, expected)
	}
}

// TestSubstituteMissingVariable ensures that placeholders for missing variables remain intact.
func TestSubstituteMissingVariable(t *testing.T) {
	store := NewStore()
	store.Set("user", "Bob") // Only one variable should exist

	input := "User: {{user}}. Secret: {{api_key}}. Unrelated: This should remain {{api_key}}."
	// Expected: An error because api_key is missing.

	_, err := Substitute(input, store, "test-step")
	if err == nil {
		t.Fatalf("Expected an error for missing variable, but got nil")
	}
	if err.Error() != "variable api_key not found in step test-step" {
		t.Errorf("Expected error message 'variable api_key not found in step test-step', got: %v", err)
	}
}

// TestSubstituteNonRecursive ensures that replacements do not trigger secondary substitutions.
func TestSubstituteNonRecursive(t *testing.T) {
	store := NewStore()
	store.Set("root_placeholder", "The result is {{A}}.")
	store.Set("A", "The internal value is {{B}}.")
	store.Set("B", "final_value.")

	// Single pass: {{A}} inside root_placeholder's value must not be expanded.
	expected := "The result is The result is {{A}}.."

	result, err := Substitute("The result is {{root_placeholder}}.", store, "test-step")
	if err != nil {
		t.Fatalf("Substitute returned an unexpected error: %v", err)
	}
	if result != expected {
		t.Errorf("Non-recursive check failed. Got: %s, Want: %s", result, expected)
	}
}

// TestSubstituteMap tests the functionality of SubstituteMap.
func TestSubstituteMap(t *testing.T) {
	store := NewStore()
	store.Set("user", "Charlie")
	store.Set("data", "JSON data")

	substitutions := map[string]string{
		"profile": "User profile: {{user}} contains {{data}}.",
	}
	expected := map[string]string{
		"profile": "User profile: Charlie contains JSON data.",
	}

	result, err := SubstituteMap(substitutions, store, "test-step")
	if err != nil {
		t.Fatalf("SubstituteMap returned an unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("SubstituteMap failed. Got: %v, Want: %v", result, expected)
	}
}

// TestSubstituteMixedFailure checks that non-existent variables are left untouched until the end.
func TestSubstituteMixedFailure(t *testing.T) {
	store := NewStore()
	store.Set("valid_key", "YES")

	input := "Start {{valid_key}}. Mid {{missing}}. End {{valid_key}}."

	_, err := Substitute(input, store, "test-step")
	if err == nil {
		t.Fatalf("Expected an error for missing variable, but got nil")
	}
}
