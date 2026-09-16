//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func buildKaktoosBinary(t *testing.T, root string) string {
	t.Helper()
	binary := filepath.Join(root, "kaktoos-impact-test")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/kaktoos")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}
	t.Cleanup(func() { os.Remove(binary) })
	return binary
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=e2e", "GIT_AUTHOR_EMAIL=e2e@example.com",
		"GIT_COMMITTER_NAME=e2e", "GIT_COMMITTER_EMAIL=e2e@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestE2E_Impact builds a real binary, sets up a fixture monorepo as an
// actual git repository, changes payment code, and confirms `kaktoos impact`
// reports the expected operation and workflow, then confirms --verify
// actually runs that workflow against a live server.
func TestE2E_Impact(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := buildKaktoosBinary(t, root)

	dir := t.TempDir()
	if out, err := exec.Command("cp", "-R", filepath.Join(root, "internal/discovery/testdata/monorepo")+"/.", dir).CombinedOutput(); err != nil {
		t.Fatalf("copy fixture: %v\n%s", err, out)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "initial commit")

	if err := os.WriteFile(filepath.Join(dir, "payment-service/fee.go"),
		[]byte("package payment\n\nfunc Fee(a int) int { return a / 50 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(binary, "impact", "--repo", dir, "--format", "json").CombinedOutput()
	if err != nil {
		t.Fatalf("impact failed: %v\n%s", err, out)
	}

	var result struct {
		Changed   []struct{ Name string } `json:"changed"`
		Potential []struct {
			Entity struct{ Kind, Name string }
		} `json:"potential_impact"`
		Verification struct {
			Workflows []struct{ Name, File string }
		} `json:"verification"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	if len(result.Changed) != 1 || result.Changed[0].Name != "payment-service/fee.go" {
		t.Fatalf("changed: %+v", result.Changed)
	}
	var sawOperation, sawWorkflow bool
	for _, p := range result.Potential {
		if p.Entity.Kind == "operation" && p.Entity.Name == "POST /payments" {
			sawOperation = true
		}
	}
	for _, w := range result.Verification.Workflows {
		if w.Name == "payment-create" {
			sawWorkflow = true
		}
	}
	if !sawOperation {
		t.Fatalf("expected POST /payments in potential impact: %+v", result.Potential)
	}
	if !sawWorkflow {
		t.Fatalf("expected payment-create in verification plan: %+v", result.Verification)
	}

	// --verify: run the plan's workflow against a real server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	envPath := filepath.Join(dir, "env.yml")
	if err := os.WriteFile(envPath, []byte(fmt.Sprintf("base_url: %s\n", srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}

	verifyOut, err := exec.Command(binary, "impact", "--repo", dir, "--verify",
		"--openapi", filepath.Join(dir, "payment-service/openapi.yml"), "--env", envPath).CombinedOutput()
	if err != nil {
		t.Fatalf("impact --verify failed: %v\n%s", err, verifyOut)
	}
}
