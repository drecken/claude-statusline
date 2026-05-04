package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/drecken/claude-statusline/internal/color"
)

type task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Label       string `json:"label"`
	StartTime   string `json:"startTime"`
	TokenCount  int    `json:"tokenCount"`
	CWD         string `json:"cwd"`
}

type input struct {
	Columns int    `json:"columns"`
	Tasks   []task `json:"tasks"`
}

type output struct {
	ID      string `json:"id"`
	Content string `json:"content"`
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

	enc := json.NewEncoder(os.Stdout)
	for _, t := range in.Tasks {
		content := renderRow(t, in.Columns)
		_ = enc.Encode(output{ID: t.ID, Content: content})
	}
}

func renderRow(t task, columns int) string {
	badge := modelBadge(t)
	typ := taskType(t)
	elapsed := elapsedOf(t.StartTime)
	desc := t.Description
	if desc == "" {
		desc = t.Label
	}
	tokens := formatTokens(t.TokenCount)

	prefix := badge + " " +
		color.Wrap(color.Cyan, typ) + " " +
		color.Wrap(color.BrightBlack, elapsed)
	suffix := " " + color.Wrap(color.Dim, tokens)

	// Truncate description to keep total width under columns, if known.
	if columns > 0 {
		budget := columns - visibleLen(prefix) - visibleLen(suffix) - 1
		if budget > 3 && visibleLen(desc) > budget {
			r := []rune(desc)
			desc = string(r[:budget-1]) + "…"
		}
	}

	return prefix + " " + desc + suffix
}

// modelBadge returns a single-char badge colored by model family. Defaults to
// 's' for Sonnet-class subagents when label is blank.
func modelBadge(t task) string {
	label := strings.ToLower(t.Label + " " + t.Name + " " + t.Type)
	switch {
	case strings.Contains(label, "opus"):
		return color.Wrap(color.Magenta, "O")
	case strings.Contains(label, "haiku"):
		return color.Wrap(color.Green, "h")
	default:
		return color.Wrap(color.Yellow, "s")
	}
}

func taskType(t task) string {
	if t.Type != "" {
		return t.Type
	}
	if t.Name != "" {
		return t.Name
	}
	return "task"
}

func elapsedOf(start string) string {
	if start == "" {
		return "--"
	}
	parsed, err := time.Parse(time.RFC3339, start)
	if err != nil {
		// Try millisecond epoch
		if ms, err2 := parseEpoch(start); err2 == nil {
			parsed = time.UnixMilli(ms)
		} else {
			return "--"
		}
	}
	d := time.Since(parsed)
	if d < 0 {
		return "0s"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func parseEpoch(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func formatTokens(n int) string {
	switch {
	case n <= 0:
		return "0t"
	case n < 1000:
		return fmt.Sprintf("%dt", n)
	case n < 10000:
		return fmt.Sprintf("%.1fkt", float64(n)/1000)
	default:
		return fmt.Sprintf("%dkt", n/1000)
	}
}

// visibleLen strips ANSI escape sequences then counts runes.
func visibleLen(s string) int {
	n := 0
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			// Skip CSI/OSC until terminator.
			j := i + 1
			// OSC 8 hyperlink: ends with BEL (0x07) or ESC\
			if j < len(s) && s[j] == ']' {
				for j < len(s) && s[j] != 0x07 {
					j++
				}
				i = j + 1
				continue
			}
			// CSI: ESC [ ... letter
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					c := s[j]
					j++
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						break
					}
				}
				i = j
				continue
			}
			i = j + 1
			continue
		}
		_, size := decodeRune(s[i:])
		n++
		i += size
	}
	return n
}

func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	c := s[0]
	switch {
	case c < 0x80:
		return rune(c), 1
	case c < 0xC0:
		return rune(c), 1
	case c < 0xE0:
		if len(s) < 2 {
			return rune(c), 1
		}
		return rune(c), 2
	case c < 0xF0:
		if len(s) < 3 {
			return rune(c), 1
		}
		return rune(c), 3
	default:
		if len(s) < 4 {
			return rune(c), 1
		}
		return rune(c), 4
	}
}
