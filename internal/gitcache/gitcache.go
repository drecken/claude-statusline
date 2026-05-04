package gitcache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	cacheTTL = 5 * time.Second
	gitTO    = 5 * time.Second
)

func cachePath(sessionID string) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	return filepath.Join(os.TempDir(), "claude-statusline-git-"+sessionID)
}

// Branch returns the current git branch for cwd, cached by sessionID for ~5s.
// Returns empty string when cwd is not a git worktree.
func Branch(cwd, sessionID string) string {
	path := cachePath(sessionID)
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
