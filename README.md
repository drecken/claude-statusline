# claude-statusline

Fast Go replacement for `ccstatusline` plus a custom subagent row renderer for
[Claude Code](https://code.claude.com/docs/en/statusline).

Two single-binary tools with no runtime dependencies beyond `git` (and
optionally `gh` / `glab`).

## Why

`bunx -y ccstatusline@latest` cold-starts cost 200–500 ms per render, and the
status line refreshes often. This replacement targets **<25 ms per render** with
warm caches and drops every feature that wasn't actually displayed.

## Output

Three lines, cyan / yellow / green scheme, OSC 8 hyperlinks on the PR number:

```
~/Code/project
main | +23/-5 | PR #142 APPROVED Fix edge case in retry logic
ctx 42% | tkn 84K | Opus 4.7 | effort:high | think
```

**Line 2 colors**

| Element      | Color                                                                 |
| ------------ | --------------------------------------------------------------------- |
| Branch       | bright-black                                                          |
| `+N` / `-N`  | green / red, dim when zero                                            |
| PR (merged)  | dim                                                                   |
| PR (closed)  | red                                                                   |
| PR (CI fail) | red (from `statusCheckRollup`)                                        |
| PR (changes) | red (`reviewDecision === "CHANGES_REQUESTED"`)                        |
| PR (approved)| green (`reviewDecision === "APPROVED"` and CI not failing)            |
| PR (open)    | yellow                                                                |
| no PR        | dim `(no PR)`                                                         |

**Line 3 colors**

| Element   | Rule                                                    |
| --------- | ------------------------------------------------------- |
| Context % | green <70, yellow 70–84, red ≥85                        |
| Tokens    | green <150K, yellow ≥150K, red ≥180K; omitted when no `current_usage` |
| Model     | dim                                                     |
| Effort    | cyan, omitted when `effort` absent                      |
| Thinking  | magenta `think`, shown only when `thinking.enabled`     |

## Subagent status line

Renders `<badge> <type> <elapsed> <description> <tokens>` per row, width-aware
truncation of the description based on `columns`. Badge is `O` (magenta, Opus),
`s` (yellow, Sonnet), or `h` (green, Haiku) derived from the task's `label` /
`name` / `type`.

## Install

### Pre-built binary (no Go required)

```bash
curl -fsSL https://raw.githubusercontent.com/drecken/claude-statusline/main/install.sh | bash
```

Downloads the latest release for your platform (darwin/linux × amd64/arm64),
verifies the checksum, installs both binaries to `$HOME/.claude/bin/`, and
prints the `settings.json` snippet to add.

Pin a version with `VERSION=v0.1.0` or change the install location with
`INSTALL_DIR=/somewhere/else`.

### Ask Claude Code to install it

Paste this prompt into Claude Code:

> Install https://github.com/drecken/claude-statusline — run `install.sh` from
> the repo, then add the printed `statusLine` and `subagentStatusLine` keys to
> my `~/.claude/settings.json`.

Claude will run the installer and patch `settings.json` for you.

### From source

Requires Go 1.26+.

```bash
go install github.com/drecken/claude-statusline/cmd/statusline@latest
go install github.com/drecken/claude-statusline/cmd/subagent-statusline@latest
```

Or clone and `make install` to put binaries in `$HOME/.claude/bin/`. Override
with `make install INSTALL_DIR=/somewhere/else`.

### Wire it up

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "/Users/YOU/.claude/bin/statusline",
    "padding": 0,
    "refreshInterval": 30
  },
  "subagentStatusLine": {
    "type": "command",
    "command": "/Users/YOU/.claude/bin/subagent-statusline"
  }
}
```

## Caching

- **PR cache** — `~/.cache/claude-statusline/git-review/git-review-<hash>.json`,
  keyed on `cwd + branch` (or short HEAD), TTL 30 s, with negative caching for
  branches without a PR. Provider auto-detects: `github*` hosts use `gh`,
  `gitlab*` use `glab`, self-hosted is probed via `auth status --hostname`.
- **Git branch cache** — `$TMPDIR/claude-statusline-git-<session_id>`, TTL 5 s.
  Session-keyed so concurrent sessions in different repos don't collide.
- **Git diff cache** — `$TMPDIR/claude-statusline-diff-<session_id>`, TTL 5 s.
  Stores `+N/-N` from `git diff --shortstat HEAD`.

Every subprocess call has a 5 s timeout.

## Stdin fields consumed

Main statusline:

| Field                            | Use                      |
| -------------------------------- | ------------------------ |
| `workspace.current_dir`          | Line 1                   |
| `session_id`                     | Branch cache key         |
| `context_window.used_percentage` | Line 3 (ctx %)           |
| `context_window.current_usage.{input_tokens,cache_creation_input_tokens,cache_read_input_tokens}` | Line 3 (tkn NK), summed |
| `model.display_name` / `model.id`| Line 3                   |
| `effort.level`                   | Line 3 (conditional)     |
| `thinking.enabled`               | Line 3 (conditional)     |

Line 2's `+N/-N` comes from `git diff --shortstat HEAD` (staged + unstaged vs
HEAD), so it drops to `+0/-0` on commit instead of accumulating across the
session.

Subagent statusline: `columns` and `tasks[]` (`id, name, type, status, description, label, startTime, tokenCount, cwd`).

## Layout

```
cmd/
  statusline/          main statusline binary
  subagent-statusline/ subagent row renderer
internal/
  color/               ANSI color codes
  gitcache/            session-keyed branch cache
  gitremote/           origin URL parser (SSH / HTTPS / ssh:// / git://)
  gitreview/           gh/glab wrapper with 30 s review cache
  hyperlink/           OSC 8 link wrapper
```

## Verify

```bash
echo '{"workspace":{"current_dir":"'$PWD'"},"session_id":"test","model":{"display_name":"Opus 4.7"},"context_window":{"used_percentage":42},"effort":{"level":"high"},"thinking":{"enabled":true}}' \
  | ./bin/statusline
```

## Acknowledgements

The PR review cache and OSC 8 hyperlink helpers are ports of
[ccstatusline](https://github.com/sirmalloc/ccstatusline) (`src/utils/git-review-cache.ts`,
`src/utils/git-remote.ts`, `src/utils/hyperlink.ts`, `src/widgets/GitPr.ts`).
