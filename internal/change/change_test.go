package change

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "fee.go"), []byte("package payment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "initial commit\n\nADD-1 sets up the repo")
	return dir
}

func paths(files []ChangedFile) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path+":"+f.Status)
	}
	sort.Strings(out)
	return out
}

func TestWorkingTreeUncommittedChanges(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "fee.go"), []byte("package payment\n\nfunc Fee() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package payment\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs, err := WorkingTree{}.Changes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(cs.Files); len(got) != 2 || got[0] != "fee.go:M" || got[1] != "new.go:A" {
		t.Fatalf("files: %v", got)
	}
	if cs.Branch != "main" {
		t.Fatalf("branch: %q", cs.Branch)
	}
}

func TestCommitChanges(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "fee.go"), []byte("package payment\n\nfunc Fee() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-q", "-am", "fix fee rounding\n\nsee PROJ-42")

	cs, err := Commit{SHA: "HEAD"}.Changes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(cs.Files); len(got) != 1 || got[0] != "fee.go:M" {
		t.Fatalf("files: %v", got)
	}
	if cs.Message == "" || !contains(cs.Message, "PROJ-42") {
		t.Fatalf("message should include commit body: %q", cs.Message)
	}
}

func TestRangeChanges(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "fee.go"), []byte("package payment\n\nfunc Fee() int { return 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "commit", "-q", "-am", "feature change")

	cs, err := Range{Base: "main", Head: "feature"}.Changes(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(cs.Files); len(got) != 1 || got[0] != "fee.go:M" {
		t.Fatalf("files: %v", got)
	}
}

func TestGitMissingIsProviderUnavailable(t *testing.T) {
	// Simulate git being unavailable by pointing PATH somewhere without it.
	t.Setenv("PATH", t.TempDir())
	_, err := WorkingTree{}.Changes(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func TestPullRequestWithoutGhIsProviderUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := PullRequest{Number: 1}.Changes(context.Background(), t.TempDir())
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }
