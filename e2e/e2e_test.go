//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_ValidateCommand(t *testing.T) {
	// Get absolute path to project root
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	// Build binary
	binary := filepath.Join(root, "kaktoos-test")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}
	defer os.Remove(binary)

	// Run validate
	cmd = exec.Command(binary, "validate",
		"--openapi", filepath.Join(root, "examples/openapi.yml"),
		"--env", filepath.Join(root, "examples/environments/local.yml"),
		"--scenario", filepath.Join(root, "examples/scenarios/test.yml"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "All files valid") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestE2E_RunCommand(t *testing.T) {
	// Start test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/users" {
			w.WriteHeader(201)
			w.Write([]byte(`{"id":"123","name":"Test User"}`))
		} else if r.Method == "GET" && r.URL.Path == "/users/123" {
			w.Write([]byte(`{"id":"123","name":"Test User"}`))
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	// Get absolute path to project root
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	// Build binary
	binary := filepath.Join(root, "kaktoos-test")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}
	defer os.Remove(binary)

	// Create test environment file with server URL
	envContent := fmt.Sprintf(`base_url: %s
variables:
  testUser: "test-user-123"
  authToken: "test-token"
headers:
  Content-Type: application/json
`, server.URL)

	envPath := filepath.Join(root, "examples/environments/test.yml")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(envPath)

	// Run scenario
	cmd = exec.Command(binary, "run",
		"--openapi", filepath.Join(root, "examples/openapi.yml"),
		"--env", envPath,
		"--scenario", filepath.Join(root, "examples/scenarios/test.yml"))
	output, err := cmd.CombinedOutput()

	// Should pass
	if err != nil {
		t.Errorf("run failed: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "PASSED") {
		t.Errorf("expected PASSED in output: %s", output)
	}
}
