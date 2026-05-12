package gitcache

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cacheTTL = 5 * time.Second
	gitTO    = 5 * time.Second
)

type Changes struct {
	Added   int
	Removed int
}

func branchCachePath(sessionID string) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	return filepath.Join(os.TempDir(), "claude-statusline-git-"+sessionID)
}

func diffCachePath(sessionID string) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	return filepath.Join(os.TempDir(), "claude-statusline-diff-"+sessionID)
}

// Branch returns the current git branch for cwd, cached by sessionID for ~5s.
// Returns empty string when cwd is not a git worktree.
func Branch(cwd, sessionID string) string {
	path := branchCachePath(sessionID)
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < cacheTTL {
		if b, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(b))
		}
	}

	branch := fetchBranch(cwd)
	_ = os.WriteFile(path, []byte(branch), 0o644)
	return branch
}

func fetchBranch(cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTO)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = cwd
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Diff returns staged+unstaged line changes vs HEAD, cached for ~5s.
// Returns zero Changes for non-git dirs and repos without HEAD.
func Diff(cwd, sessionID string) Changes {
	path := diffCachePath(sessionID)
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < cacheTTL {
		if b, err := os.ReadFile(path); err == nil {
			if c, ok := parseCache(string(b)); ok {
				return c
			}
		}
	}

	c := fetchDiff(cwd)
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d %d", c.Added, c.Removed)), 0o644)
	return c
}

func parseCache(s string) (Changes, bool) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return Changes{}, false
	}
	a, err1 := strconv.Atoi(fields[0])
	r, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return Changes{}, false
	}
	return Changes{Added: a, Removed: r}, true
}

func fetchDiff(cwd string) Changes {
	ctx, cancel := context.WithTimeout(context.Background(), gitTO)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--shortstat", "HEAD")
	cmd.Dir = cwd
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return Changes{}
	}
	return parseShortstat(string(out))
}

func parseShortstat(s string) Changes {
	s = strings.TrimSpace(s)
	if s == "" {
		return Changes{}
	}
	var c Changes
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.Contains(part, "insertion"):
			c.Added = leadingInt(part)
		case strings.Contains(part, "deletion"):
			c.Removed = leadingInt(part)
		}
	}
	return c
}

func leadingInt(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}
