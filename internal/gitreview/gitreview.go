package gitreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/drecken/claude-statusline/internal/gitremote"
)

type Provider string

const (
	ProviderGH   Provider = "gh"
	ProviderGLab Provider = "glab"
)

type CheckRun struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

type Data struct {
	Number         int        `json:"number"`
	URL            string     `json:"url"`
	Title          string     `json:"title"`
	State          string     `json:"state"`
	ReviewDecision string     `json:"reviewDecision"`
	Checks         []CheckRun `json:"statusCheckRollup,omitempty"`
	Provider       Provider   `json:"provider,omitempty"`
}

const (
	cacheTTL   = 30 * time.Second
	cliTimeout = 5 * time.Second
)

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude-statusline", "git-review")
}

func cachePath(cwd, ref string) string {
	sum := sha256.New()
	sum.Write([]byte(cwd))
	sum.Write([]byte{0})
	sum.Write([]byte(ref))
	return filepath.Join(cacheDir(), "git-review-"+hex.EncodeToString(sum.Sum(nil))[:16]+".json")
}

func runGit(cwd string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func currentBranch(cwd string) string {
	return runGit(cwd, "branch", "--show-current")
}

func cacheRef(cwd string) string {
	if b := currentBranch(cwd); b != "" {
		return "branch:" + b
	}
	if h := runGit(cwd, "rev-parse", "--short", "HEAD"); h != "" {
		return "head:" + h
	}
	return "unknown"
}

// readCache returns (data, hit). hit=true means don't refetch (data may be nil
// for negative-cache entries). hit=false means cache miss/stale/corrupt.
func readCache(path string) (*Data, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(st.ModTime()) > cacheTTL {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	content := strings.TrimSpace(string(b))
	if content == "" {
		return nil, true // negative cache
	}
	var d Data
	if err := json.Unmarshal([]byte(content), &d); err != nil {
		return nil, false
	}
	if d.Number == 0 || d.URL == "" {
		return nil, false
	}
	return &d, true
}

func writeCache(path string, d *Data) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if d == nil {
		_ = os.WriteFile(path, []byte{}, 0o644)
		return
	}
	b, err := json.Marshal(d)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func originURL(cwd string) string {
	return runGit(cwd, "remote", "get-url", "--", "origin")
}

func originHost(cwd string) string {
	u := originURL(cwd)
	if u == "" {
		return ""
	}
	r := gitremote.Parse(u)
	if r == nil {
		return ""
	}
	return strings.ToLower(r.Host)
}

func originRepoRef(cwd string) string {
	u := originURL(cwd)
	if u == "" {
		return ""
	}
	r := gitremote.Parse(u)
	if r == nil {
		return ""
	}
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Owner, r.Repo)
}

func providerCandidates(cwd string) []Provider {
	host := originHost(cwd)
	if host == "" {
		return []Provider{ProviderGH, ProviderGLab}
	}
	if strings.Contains(host, "github") {
		return []Provider{ProviderGH}
	}
	if strings.Contains(host, "gitlab") {
		return []Provider{ProviderGLab}
	}
	var out []Provider
	if isCliAuthed(ProviderGLab, host) {
		out = append(out, ProviderGLab)
	}
	if isCliAuthed(ProviderGH, host) {
		out = append(out, ProviderGH)
	}
	return out
}

func cliAvailable(cli Provider) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	return exec.CommandContext(ctx, string(cli), "--version").Run() == nil
}

func isCliAuthed(cli Provider, host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	return exec.CommandContext(ctx, string(cli), "auth", "status", "--hostname", host).Run() == nil
}

func runGH(cwd string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = cwd
	cmd.Stderr = nil
	return cmd.Output()
}

func runGlab(cwd string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Dir = cwd
	cmd.Stderr = nil
	return cmd.Output()
}

func fetchFromGH(cwd, repoRef string) (*Data, error) {
	args := []string{"pr", "view"}
	if repoRef != "" {
		branch := currentBranch(cwd)
		if branch == "" {
			return nil, nil
		}
		args = append(args, branch, "--repo", repoRef)
	}
	args = append(args, "--json", "url,number,title,state,reviewDecision,statusCheckRollup")
	out, err := runGH(cwd, args...)
	if err != nil {
		return nil, err
	}
	out = []byte(strings.TrimSpace(string(out)))
	if len(out) == 0 {
		return nil, nil
	}
	var d Data
	if err := json.Unmarshal(out, &d); err != nil {
		return nil, err
	}
	if d.Number == 0 || d.URL == "" {
		return nil, nil
	}
	d.Provider = ProviderGH
	return &d, nil
}

func mapGlabState(s string) string {
	switch s {
	case "opened":
		return "OPEN"
	case "closed":
		return "CLOSED"
	case "merged":
		return "MERGED"
	case "locked":
		return "LOCKED"
	default:
		return strings.ToUpper(s)
	}
}

func fetchFromGlab(cwd, repoRef string) (*Data, error) {
	args := []string{"mr", "view"}
	if repoRef != "" {
		branch := currentBranch(cwd)
		if branch == "" {
			return nil, nil
		}
		args = append(args, branch, "--repo", repoRef)
	}
	args = append(args, "--output", "json")
	out, err := runGlab(cwd, args...)
	if err != nil {
		return nil, err
	}
	out = []byte(strings.TrimSpace(string(out)))
	if len(out) == 0 {
		return nil, nil
	}
	var raw struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	if raw.IID == 0 || raw.WebURL == "" {
		return nil, nil
	}
	return &Data{
		Number:   raw.IID,
		URL:      raw.WebURL,
		Title:    raw.Title,
		State:    mapGlabState(raw.State),
		Provider: ProviderGLab,
	}, nil
}

func fetchFromProvider(p Provider, cwd, repoRef string) (*Data, error) {
	fetch := fetchFromGH
	if p == ProviderGLab {
		fetch = fetchFromGlab
	}
	if d, err := fetch(cwd, ""); err == nil && d != nil {
		return d, nil
	}
	if repoRef != "" {
		return fetch(cwd, repoRef)
	}
	return nil, nil
}

// Fetch returns PR/MR info for the current branch in cwd, with a 30s cache
// (including negative caching for no-PR branches).
func Fetch(cwd string) *Data {
	path := cachePath(cwd, cacheRef(cwd))
	if d, hit := readCache(path); hit {
		return d
	}

	repoRef := originRepoRef(cwd)

	for _, p := range providerCandidates(cwd) {
		if !cliAvailable(p) {
			continue
		}
		if d, err := fetchFromProvider(p, cwd, repoRef); err == nil && d != nil {
			writeCache(path, d)
			return d
		}
	}
	writeCache(path, nil)
	return nil
}

// StatusLabel maps state+reviewDecision to a short label matching ccstatusline.
func StatusLabel(state, reviewDecision string) string {
	switch state {
	case "MERGED":
		return "MERGED"
	case "CLOSED":
		return "CLOSED"
	}
	switch reviewDecision {
	case "APPROVED":
		return "APPROVED"
	case "CHANGES_REQUESTED":
		return "CHANGES_REQ"
	}
	if state == "OPEN" {
		return "OPEN"
	}
	return state
}

// TruncateTitle shortens a PR title using an ellipsis character if it exceeds
// the 30-char budget used by ccstatusline.
func TruncateTitle(title string) string {
	const maxWidth = 30
	if len([]rune(title)) <= maxWidth {
		return title
	}
	r := []rune(title)
	return string(r[:maxWidth-1]) + "…"
}

// HasFailedCheck returns true if any check in the rollup concluded FAILURE.
func HasFailedCheck(d *Data) bool {
	if d == nil {
		return false
	}
	for _, c := range d.Checks {
		if strings.EqualFold(c.Conclusion, "FAILURE") || strings.EqualFold(c.Conclusion, "TIMED_OUT") || strings.EqualFold(c.Conclusion, "CANCELLED") {
			return true
		}
	}
	return false
}
