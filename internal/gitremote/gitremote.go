package gitremote

import (
	"net/url"
	"regexp"
	"strings"
)

type Remote struct {
	Host  string
	Owner string
	Repo  string
}

var sshRe = regexp.MustCompile(`^(?:[^@]+@)?([^:]+):(.+?)(?:\.git)?/?$`)

// Parse extracts host/owner/repo from a git remote URL. Supports SSH, HTTPS,
// git:// and ssh:// forms.
func Parse(raw string) *Remote {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	if !strings.Contains(trimmed, "://") {
		if m := sshRe.FindStringSubmatch(trimmed); m != nil {
			segs := splitNonEmpty(m[2], "/")
			if len(segs) < 2 {
				return nil
			}
			repo := segs[len(segs)-1]
			owner := strings.Join(segs[:len(segs)-1], "/")
			repo = strings.TrimSuffix(repo, ".git")
			if owner == "" || repo == "" {
				return nil
			}
			return &Remote{Host: m[1], Owner: owner, Repo: repo}
		}
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return nil
	}
	switch u.Scheme {
	case "http", "https", "ssh", "git":
	default:
		return nil
	}
	pathname := strings.Trim(u.Path, "/")
	pathname = strings.TrimSuffix(pathname, ".git")
	segs := splitNonEmpty(pathname, "/")
	if len(segs) < 2 {
		return nil
	}
	repo := segs[len(segs)-1]
	owner := strings.Join(segs[:len(segs)-1], "/")
	if owner == "" || repo == "" {
		return nil
	}
	host := u.Host
	if u.Scheme == "ssh" || u.Scheme == "git" {
		host = u.Hostname()
	}
	return &Remote{Host: host, Owner: owner, Repo: repo}
}

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
