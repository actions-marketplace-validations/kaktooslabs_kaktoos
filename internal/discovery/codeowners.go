package discovery

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// CodeownersLocations are checked in order, mirroring GitHub's own lookup.
var CodeownersLocations = []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}

// Codeowners is a parsed CODEOWNERS file: gitignore-style rules, last match
// wins. A path matching no rule has no owners — Kaktoos never guesses one.
type Codeowners struct {
	rules []codeownersRule
}

type codeownersRule struct {
	pattern string
	owners  []string
}

// LoadCodeowners finds and parses the repository's CODEOWNERS file, if any.
// A missing file is not an error: it returns an empty (always-no-owners) set.
func LoadCodeowners(root string) (*Codeowners, error) {
	for _, loc := range CodeownersLocations {
		p := filepath.Join(root, loc)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return ParseCodeowners(string(data)), nil
	}
	return &Codeowners{}, nil
}

// ParseCodeowners reads CODEOWNERS syntax: "<pattern> <owner> [<owner>...]"
// per line, blank lines and "#" comments ignored.
func ParseCodeowners(data string) *Codeowners {
	c := &Codeowners{}
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		c.rules = append(c.rules, codeownersRule{pattern: fields[0], owners: fields[1:]})
	}
	return c
}

// Owners returns the owners of a repo-relative path. Last matching rule wins,
// same as gitignore precedence. No match => no owners.
func (c *Codeowners) Owners(path string) []string {
	path = filepath.ToSlash(path)
	var owners []string
	for _, r := range c.rules {
		if codeownersMatch(r.pattern, path) {
			owners = r.owners
		}
	}
	return owners
}

// codeownersMatch reports whether a gitignore-style pattern covers path.
// Supports the subset CODEOWNERS actually uses: a trailing "/**" or "/"
// matches the directory and everything under it; otherwise the pattern must
// match the path itself or one of its leading directory segments via
// filepath.Match, or be a bare prefix directory.
func codeownersMatch(pattern, path string) bool {
	pattern = strings.TrimPrefix(pattern, "/")
	path = strings.TrimPrefix(path, "/")

	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "/**") {
		dir := strings.TrimSuffix(pattern, "/**")
		return path == dir || strings.HasPrefix(path, dir+"/")
	}
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		return path == dir || strings.HasPrefix(path, dir+"/")
	}

	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	// A pattern with no "/" matches by file name at any depth ("*.md").
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	// A pattern with no wildcard and no trailing slash still owns everything
	// under it when it names a directory prefix (e.g. "payment-service").
	if !strings.ContainsAny(pattern, "*?[") {
		if path == pattern || strings.HasPrefix(path, pattern+"/") {
			return true
		}
	}
	return false
}
