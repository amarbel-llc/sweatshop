# Update Command Design

## Summary

Add an `update` subcommand that pulls all repos and rebases all worktrees in two phases. Output is TAP-14 via tap-dancer. A `-d/--dirty` flag includes dirty repos and worktrees.

## Behavior

### Phase 1: Pull repos

Scan `$HOME/eng*/repos/` for git repositories. For each repo:

- **Clean:** run `git pull` (current branch's upstream).
- **Dirty, `--dirty` set:** run `git pull` anyway.
- **Dirty, no `--dirty`:** skip, emit `ok N - pull <label> # SKIP dirty`.

### Phase 2: Rebase worktrees

After all pulls complete, scan `$HOME/eng*/worktrees/<repo>/*/`. For each worktree:

- Determine the parent repo's default branch via `git branch --show-current` on the repo.
- **Clean:** run `git rebase <default-branch>`.
- **Dirty, `--dirty` set:** attempt `git rebase <default-branch>` anyway.
- **Dirty, no `--dirty`:** skip, emit `ok N - rebase <label> # SKIP dirty`.

All eng areas are included (no eng* prefix filtering like status does).

### Failure handling

Failed pulls or rebases emit `not ok` with tap-dancer YAML diagnostics:

```
not ok 4 - pull eng/repos/grit
  ---
  message: "conflict on merge"
  severity: fail
  ...
```

### TAP-14 output

Always TAP-14 (no `--format` flag). Uses `github.com/amarbel-llc/tap-dancer/go` for spec-compliant output with proper quoting and severity fields. Trailing plan (`1..N` after all tests).

Example:

```
TAP version 14
ok 1 - pull eng/repos/dodger
ok 2 - pull eng/repos/lux
ok 3 - pull eng/repos/sweatshop # SKIP dirty
not ok 4 - pull eng/repos/grit
  ---
  message: "conflict on merge"
  severity: fail
  ...
ok 5 - rebase eng/worktrees/dodger/feature-x
ok 6 - rebase eng/worktrees/lux/fix-bug # SKIP dirty
1..6
```

## Package structure

- `internal/update/update.go` — `Run(home string, dirty bool) error`
- `internal/git/git.go` — add `Pull(repoPath) (string, error)` and `Rebase(repoPath, onto string) (string, error)`
- `cmd/sweatshop/main.go` and `cmd/spinclass/main.go` — register `updateCmd` with `-d/--dirty` flag

## Scope

- Repos: `git pull` on current branch upstream
- Worktrees: `git rebase <repo-default-branch>` after all pulls complete
- Dirty detection: `git status --porcelain` (same as status/clean commands)
- No table output, no `--format` flag
- No interactive prompts
