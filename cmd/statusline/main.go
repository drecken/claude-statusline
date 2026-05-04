package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/drecken/claude-statusline/internal/color"
	"github.com/drecken/claude-statusline/internal/gitcache"
	"github.com/drecken/claude-statusline/internal/gitreview"
	"github.com/drecken/claude-statusline/internal/hyperlink"
)

type input struct {
	SessionID string `json:"session_id"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	Effort *struct {
		Level string `json:"level"`
	} `json:"effort"`
	Thinking *struct {
		Enabled bool `json:"enabled"`
	} `json:"thinking"`
	Cost struct {
		TotalLinesAdded   int `json:"total_lines_added"`
		TotalLinesRemoved int `json:"total_lines_removed"`
	} `json:"cost"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return
	}
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return
	}

	cwd := in.Workspace.CurrentDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	fmt.Println(renderLine1(cwd))
	fmt.Println(renderLine2(cwd, in.SessionID, in.Cost.TotalLinesAdded, in.Cost.TotalLinesRemoved))
	fmt.Println(renderLine3(in))
}

func renderLine1(cwd string) string {
	home, _ := os.UserHomeDir()
	display := cwd
	if home != "" && strings.HasPrefix(cwd, home) {
		display = "~" + strings.TrimPrefix(cwd, home)
	}
	return color.Wrap(color.Cyan, display)
}

func renderLine2(cwd, sessionID string, added, removed int) string {
	branch := gitcache.Branch(cwd, sessionID)
	if branch == "" {
		return color.Wrap(color.Dim, "(no git)")
	}

	parts := []string{color.Wrap(color.BrightBlack, branch), renderChanges(added, removed)}

	pr := gitreview.Fetch(cwd)
	parts = append(parts, renderPR(pr))

	return strings.Join(parts, " "+color.Wrap(color.BrightBlack, "|")+" ")
}

func renderChanges(added, removed int) string {
	if added == 0 && removed == 0 {
		return color.Wrap(color.Dim, "+0/-0")
	}
	addPart := color.Wrap(color.Green, fmt.Sprintf("+%d", added))
	if added == 0 {
		addPart = color.Wrap(color.Dim, "+0")
	}
	remPart := color.Wrap(color.Red, fmt.Sprintf("-%d", removed))
	if removed == 0 {
		remPart = color.Wrap(color.Dim, "-0")
	}
	return addPart + "/" + remPart
}

func renderPR(pr *gitreview.Data) string {
	if pr == nil {
		return color.Wrap(color.Dim, "(no PR)")
	}
	noun := prNoun(pr)
	label := gitreview.StatusLabel(pr.State, pr.ReviewDecision)
	title := gitreview.TruncateTitle(pr.Title)
	text := fmt.Sprintf("%s #%d", noun, pr.Number)
	if label != "" {
		text += " " + label
	}
	if title != "" {
		text += " " + title
	}

	linked := hyperlink.Osc8(pr.URL, text)
	return color.Wrap(prColor(pr), linked)
}

func prNoun(pr *gitreview.Data) string {
	if pr.Provider == gitreview.ProviderGLab {
		return "MR"
	}
	if pr.Provider == gitreview.ProviderGH {
		return "PR"
	}
	lower := strings.ToLower(pr.URL)
	if strings.Contains(lower, "/-/merge_requests/") || strings.Contains(lower, "gitlab") {
		return "MR"
	}
	return "PR"
}

func prColor(pr *gitreview.Data) string {
	switch pr.State {
	case "MERGED":
		return color.Dim
	case "CLOSED":
		return color.Red
	}
	if gitreview.HasFailedCheck(pr) {
		return color.Red
	}
	if pr.ReviewDecision == "CHANGES_REQUESTED" {
		return color.Red
	}
	if pr.ReviewDecision == "APPROVED" {
		return color.Green
	}
	return color.Yellow
}

func renderLine3(in input) string {
	parts := []string{renderContext(in), renderModel(in)}
	if in.Effort != nil && in.Effort.Level != "" {
		parts = append(parts, color.Wrap(color.Cyan, "effort:"+in.Effort.Level))
	}
	if in.Thinking != nil && in.Thinking.Enabled {
		parts = append(parts, color.Wrap(color.Magenta, "think"))
	}
	return strings.Join(parts, " "+color.Wrap(color.BrightBlack, "|")+" ")
}

func renderContext(in input) string {
	var pct float64
	if in.ContextWindow.UsedPercentage != nil {
		pct = *in.ContextWindow.UsedPercentage
	}
	c := color.Green
	switch {
	case pct >= 85:
		c = color.Red
	case pct >= 70:
		c = color.Yellow
	}
	return color.Wrap(c, fmt.Sprintf("ctx %d%%", int(pct)))
}

func renderModel(in input) string {
	name := in.Model.DisplayName
	if name == "" {
		name = friendlyFromID(in.Model.ID)
	}
	if name == "" {
		name = "?"
	}
	return color.Wrap(color.Dim, name)
}

// friendlyFromID converts a Bedrock/Anthropic model ID like
// "us.anthropic.claude-opus-4-7" to "Opus 4.7".
func friendlyFromID(id string) string {
	if id == "" {
		return ""
	}
	lower := strings.ToLower(id)
	family := ""
	switch {
	case strings.Contains(lower, "opus"):
		family = "Opus"
	case strings.Contains(lower, "sonnet"):
		family = "Sonnet"
	case strings.Contains(lower, "haiku"):
		family = "Haiku"
	}
	if family == "" {
		return id
	}
	// Find "4-7" / "4-6" style version suffix.
	parts := strings.Split(lower, "-")
	for i, p := range parts {
		if p == "opus" || p == "sonnet" || p == "haiku" {
			if i+2 < len(parts) {
				return fmt.Sprintf("%s %s.%s", family, parts[i+1], parts[i+2])
			}
			if i+1 < len(parts) {
				return fmt.Sprintf("%s %s", family, parts[i+1])
			}
		}
	}
	return family
}

