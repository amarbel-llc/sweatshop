# Claude Allow: First-Class Permission Injection

## Goal

Replace the inlined JSON blob in the eng-area sweatfile with a first-class
`claude_allow` field. Scope `Edit` and `Write` to the worktree directory. Fix
path syntax to match Claude Code's actual parser.

## Motivation

The eng-area sweatfile currently inlines a full `settings.local.json` as a
JSON string inside a TOML `[files]` entry. This is awkward to maintain (JSON
inside TOML quoting), doesn't support per-repo customization without
duplicating the entire blob, and uses the wrong path syntax (`/*` instead of
Claude Code's `//path/**`).

Additionally, bare `Edit` is in the allow list, meaning Claude can edit files
anywhere on disk --- not just in the worktree. Edit and Write should be
strictly scoped to the worktree directory.

## Design

### New `claude_allow` sweatfile field

Add a `ClaudeAllow []string` field to the `Sweatfile` struct:

```toml
claude_allow = [
  "Read",
  "Glob",
  "Grep",
  "WebSearch",
  "Bash(git add:*)",
  "mcp__plugin_grit_grit__status",
]
```

Merge semantics follow the same array convention as `git_excludes` and
`setup`: nil inherits from eng-area, `[]` clears parent rules, non-empty
appends to parent rules.

### Auto-injected worktree-scoped rules

Two rules are always appended at create time using correct Claude Code path
syntax (double-slash prefix for absolute, double-star for recursive):

- `Edit(//worktree-path/**)`
- `Write(//worktree-path/**)`

These are not configurable and not present in any sweatfile. Bare `Read` comes
from the sweatfile (intentionally unrestricted). Bare `Edit` is never in the
sweatfile.

### Settings generation moves to sweatfile apply

`sweatfile.Apply()` gains a new step that builds `.claude/settings.local.json`
from the merged `claude_allow` rules plus the two auto-injected scoped rules.
The worktree path is passed to `Apply()` so it can generate the scoped rules.

This replaces both:

- The `[files."claude/settings.local.json"]` entry in the eng-area sweatfile
- The `injectWorktreePerms()` function in `internal/worktree/worktree.go`

### Dead code removal

- Remove `injectWorktreePerms()` and `appendUnique()` from
  `internal/worktree/worktree.go`
- Remove the `injectWorktreePerms()` call from `worktree.Create()`

The `worktree.Create()` function passes the worktree path through to
`sweatfile.Apply()` (which it already receives). The tier files and hook code
in `internal/perms/` are not touched --- they are a separate concern.

### Eng-area sweatfile update

The `~/eng/sweatfile` replaces the `[files."claude/settings.local.json"]`
block with a `claude_allow` array containing the same rules, minus bare
`Edit`. The `[files.envrc]` and other entries are unchanged.

## Scope summary

| Change | Effect |
|--------|--------|
| Add `ClaudeAllow` to `Sweatfile` struct | New TOML field, merge logic |
| Settings generation in `Apply()` | Builds JSON from rules + scoped paths |
| Remove `injectWorktreePerms()` | Dead code in `worktree.go` |
| Remove `[files."claude/settings.local.json"]` | Sweatfile cleanup |
| Fix path syntax | `//path/**` instead of `/path/*` |
| Scope Edit to worktree | Bare `Edit` removed, scoped `Edit` injected |

## Files changed

- `internal/sweatfile/sweatfile.go` --- add `ClaudeAllow` field + merge logic
- `internal/sweatfile/apply.go` --- add settings generation step
- `internal/worktree/worktree.go` --- remove `injectWorktreePerms`, pass
  worktree path to `Apply`

## Out of scope

- Permission hook registration / validation
- Tier file rework
- `perms review` command changes
- Per-repo sweatfile examples
