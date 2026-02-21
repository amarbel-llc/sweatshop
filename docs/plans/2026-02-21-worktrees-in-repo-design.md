# Worktrees Inside Repo Directory

## Summary

Move worktree storage from `~/<eng_area>/worktrees/<repo>/<branch>` to
`<repo>/.worktrees/<branch>`. Drop the convention path syntax in favor of
filesystem paths. Drop SSH/remote support. Make all scanning PWD-relative.

## Motivation

- Switching between worktree and main repo requires jumping between distant
  paths (`~/eng/repos/myrepo` vs `~/eng/worktrees/myrepo/feature-x`)
- Shell tab completion is poor across these disjoint paths
- The convention path syntax (`eng/worktrees/myrepo/feature-x`) is an extra
  concept to learn when filesystem paths would suffice

## New Path Model

**Layout:** `<repo>/.worktrees/<branch>`

**ResolvedPath struct:**

```go
ResolvedPath {
    AbsPath    string  // /home/user/eng/repos/myrepo/.worktrees/feature-x
    RepoPath   string  // /home/user/eng/repos/myrepo
    SessionKey string  // myrepo/feature-x
    Branch     string  // feature-x
}
```

Removed fields: `EngAreaDir`, `Convention`.

**Argument resolution for create/attach:**

1. If arg contains `/` or `.` -- treat as filesystem path, resolve to absolute
2. Otherwise -- treat as branch name; detect repo from PWD (walk up to `.git`),
   construct `<repo>/.worktrees/<branch>`
3. Derive `RepoPath` from parent of `.worktrees/`
4. `SessionKey` = `<repo-dirname>/<branch>`

## Discovery & Scanning

All scanning is PWD-relative. Used by status, clean, pull, completions.

1. Start from PWD
2. If PWD is a git repo (has `.git`), check for `.worktrees/` and report that
   repo's worktrees only
3. If PWD is not a repo, scan immediate children for git repos with `.worktrees/`
   dirs
4. For each repo, list `.worktrees/` entries and filter with `IsWorktree` check

No recursive deep scan -- one level of children only.

## Command Changes

### create

- Accepts branch name (`feature-x`) or path (`.worktrees/feature-x`, absolute)
- Branch name: detects repo from PWD, creates `<repo>/.worktrees/<branch>`
- Path: resolves to absolute, derives repo from parent of `.worktrees/`
- Applies sweatfile settings + claude trust
- On first create for a repo, adds `.worktrees/` to `.git/info/exclude`

### attach

- Same arg handling as create (branch name or path)
- Creates worktree if needed, launches zmx session
- No remote/SSH support

### status

- PWD-relative scanning
- Same table output

### merge

- Detects repo and branch from PWD via git (remove convention path detection)

### clean

- PWD-relative scanning, same merge-status checking

### pull

- PWD-relative scanning, same rebase logic

### completions

- Local only (no remote)
- PWD-relative: list branches under `.worktrees/` for repos found from PWD

## Removed

- `ParsePath()`, `PathComponents`, `ShopKey()` -- convention path parsing
- `ParseTarget()` -- remote host detection
- Remote completions (`ListRemote`)
- `attach.go` remote path handling
- `EngAreaDir` and `Convention` from `ResolvedPath`

## Sweatfile Loading

Merge order (each layer overrides the previous):

1. `~/.config/sweatshop/sweatfile` (global)
2. Parent directories walking down from highest ancestor to repo dir
3. `<repo>/sweatfile`

Replaces the current `LoadMerged(engAreaDir, repoDir)` which only loaded from
eng area + repo.

## .gitignore Handling

`.worktrees/` lives inside the repo directory. On first `create` for a repo,
sweatshop adds `.worktrees/` to `.git/info/exclude` so it is ignored locally
without modifying committed `.gitignore`.

## Session Key

Format: `<repo-dirname>/<branch>` (e.g. `myrepo/feature-x`).

Potential collision if two repos share a directory name in different locations.
Acceptable given PWD-scoped usage -- revisit if needed later.

## Testing

### Unit tests (worktree)

- Remove convention path parsing tests
- Add: branch-name arg resolution, path arg resolution, repo detection from PWD
- Add: session key generation

### Unit tests (scanning)

- PWD-is-repo: finds `.worktrees/` entries
- PWD-is-parent: finds child repos with `.worktrees/`
- No worktrees found

### Unit tests (sweatfile)

- Merge order: global -> parent dirs -> repo
- Partial configs

### Completions tests

- Remove remote completion tests
- Update local tests for `.worktrees/` paths and PWD-relative scanning

### Bats integration tests

- Update test_status.bats and test_completions.bats for new layout
- Test create with branch name from inside repo
- Test create with explicit path

## Migration

No automatic migration. Users with existing `~/eng/worktrees/` layouts clean up
manually.
