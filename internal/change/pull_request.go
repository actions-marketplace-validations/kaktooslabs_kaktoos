package change

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PullRequest fetches a GitHub PR's changed files via the gh CLI.
type PullRequest struct{ Number int }

type ghPRView struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	HeadRefName string `json:"headRefName"`
	Files       []struct {
		Path string `json:"path"`
	} `json:"files"`
}

func (p PullRequest) Changes(ctx context.Context, repo string) (ChangeSet, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return ChangeSet{}, fmt.Errorf("%w: gh is not installed", ErrProviderUnavailable)
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(p.Number),
		"--json", "files,title,body,headRefName")
	cmd.Dir = repo
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "auth") || strings.Contains(stderr.String(), "not logged") {
			return ChangeSet{}, fmt.Errorf("%w: gh is not authenticated: %s", ErrProviderUnavailable, strings.TrimSpace(stderr.String()))
		}
		return ChangeSet{}, fmt.Errorf("gh pr view %d: %w: %s", p.Number, err, strings.TrimSpace(stderr.String()))
	}

	var v ghPRView
	if err := json.Unmarshal(out, &v); err != nil {
		return ChangeSet{}, fmt.Errorf("gh pr view %d: %w", p.Number, err)
	}

	files := make([]ChangedFile, 0, len(v.Files))
	for _, f := range v.Files {
		files = append(files, ChangedFile{Path: f.Path, Status: "M"})
	}
	message := v.Title
	if v.Body != "" {
		message += "\n" + v.Body
	}
	return ChangeSet{
		Ref:     fmt.Sprintf("PR #%d", p.Number),
		Files:   files,
		Message: message,
		Branch:  v.HeadRefName,
	}, nil
}
