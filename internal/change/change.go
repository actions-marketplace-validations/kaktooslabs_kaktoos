// Package change reports what a diff actually changed — nothing more. It
// shells out to the git and gh binaries the developer already has rather than
// adding a git library, and it never interprets the change; that is
// internal/impact's job.
package change

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrProviderUnavailable means the tool a source needs is missing or not
// authenticated. Callers should report it as "unavailable", not as a failure
// of the analysis itself.
var ErrProviderUnavailable = errors.New("change provider unavailable")

// ChangedFile is one path touched by a change.
type ChangedFile struct {
	Path string `json:"path"`
	// Status is git's name-status letter: A added, M modified, D deleted,
	// R renamed, and so on.
	Status string `json:"status"`
	// PrevPath is set for renames.
	PrevPath string `json:"prev_path,omitempty"`
}

// ChangeSet is the factual record of a change: which files, at which ref.
type ChangeSet struct {
	Ref     string        `json:"ref"`
	Files   []ChangedFile `json:"files"`
	Message string        `json:"message,omitempty"`
	Branch  string        `json:"branch,omitempty"`
}

// Source produces a ChangeSet. Repo is the repository root to run in.
type Source interface {
	Changes(ctx context.Context, repo string) (ChangeSet, error)
}

// WorkingTree is the uncommitted change in the repository.
type WorkingTree struct{}

func (WorkingTree) Changes(ctx context.Context, repo string) (ChangeSet, error) {
	out, err := git(ctx, repo, "diff", "--name-status", "HEAD")
	if err != nil {
		return ChangeSet{}, err
	}
	files := parseNameStatus(out)

	// Untracked files are part of the working tree too, and git diff omits them.
	if untracked, err := git(ctx, repo, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, line := range splitLines(untracked) {
			files = append(files, ChangedFile{Path: line, Status: "A"})
		}
	}

	cs := ChangeSet{Ref: "working tree", Files: files}
	if branch, err := git(ctx, repo, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		cs.Branch = strings.TrimSpace(branch)
	}
	return cs, nil
}

// Commit is a single commit by SHA (or any revision git accepts).
type Commit struct{ SHA string }

func (c Commit) Changes(ctx context.Context, repo string) (ChangeSet, error) {
	out, err := git(ctx, repo, "show", "--name-status", "--format=%s%n%b", c.SHA)
	if err != nil {
		return ChangeSet{}, err
	}
	message, rest := splitCommitOutput(out)
	return ChangeSet{Ref: c.SHA, Files: parseNameStatus(rest), Message: message}, nil
}

// Range is the diff between two revisions, as a branch comparison
// (base...head) so it reflects what the head branch added.
type Range struct{ Base, Head string }

func (r Range) Changes(ctx context.Context, repo string) (ChangeSet, error) {
	out, err := git(ctx, repo, "diff", "--name-status", r.Base+"..."+r.Head)
	if err != nil {
		return ChangeSet{}, err
	}
	return ChangeSet{Ref: r.Base + "..." + r.Head, Files: parseNameStatus(out), Branch: r.Head}, nil
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("%w: git is not installed", ErrProviderUnavailable)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// parseNameStatus reads `git ... --name-status` output. Lines that are not
// tab-separated status records (commit headers, blank lines) are skipped.
func parseNameStatus(out string) []ChangedFile {
	var files []ChangedFile
	for _, line := range splitLines(out) {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		if status == "" || !isStatusCode(status) {
			continue
		}
		f := ChangedFile{Status: status[:1], Path: fields[1]}
		if strings.HasPrefix(status, "R") && len(fields) >= 3 {
			f.PrevPath = fields[1]
			f.Path = fields[2]
		}
		files = append(files, f)
	}
	return files
}

func isStatusCode(s string) bool {
	switch s[0] {
	case 'A', 'M', 'D', 'R', 'C', 'T', 'U':
		return true
	}
	return false
}

// splitCommitOutput separates the leading commit message from the trailing
// name-status block produced by `git show --name-status --format=%s%n%b`.
func splitCommitOutput(out string) (message, rest string) {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 && fields[0] != "" && isStatusCode(fields[0]) {
			return strings.TrimSpace(strings.Join(lines[:i], "\n")), strings.Join(lines[i:], "\n")
		}
	}
	return strings.TrimSpace(out), ""
}

func splitLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimRight(l, "\r"); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
