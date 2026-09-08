package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionCommand tests that the version command outputs the version.
func TestVersionCommand(t *testing.T) {
	// Save original version and restore after test
	originalVersion := version
	defer func() { version = originalVersion }()

	// Set test version
	version = "v1.2.3"

	// Capture output
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	versionCmd.SetErr(&buf)

	// Execute directly
	versionCmd.Run(versionCmd, []string{})

	// Verify output
	output := buf.String()
	expected := "v1.2.3\n"
	if output != expected {
		t.Errorf("Expected output %q, got: %q", expected, output)
	}
}

// TestVersionCommandDefaultDev tests that the default version is "dev".
func TestVersionCommandDefaultDev(t *testing.T) {
	// Save original version and restore after test
	originalVersion := version
	defer func() { version = originalVersion }()

	// Reset to default
	version = "dev"

	// Capture output
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	versionCmd.SetErr(&buf)

	// Execute directly
	versionCmd.Run(versionCmd, []string{})

	// Verify output
	output := buf.String()
	expected := "dev\n"
	if output != expected {
		t.Errorf("Expected output %q, got: %q", expected, output)
	}
}

// TestVersionCommandInHelp tests that version command appears in help.
func TestVersionCommandInHelp(t *testing.T) {
	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--help"})

	// Execute
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	// Verify version command is listed
	output := buf.String()
	if !strings.Contains(output, "version") {
		t.Error("Expected help output to list 'version' command")
	}
	if !strings.Contains(output, "Print the version number") {
		t.Error("Expected help output to show version command description")
	}
}
