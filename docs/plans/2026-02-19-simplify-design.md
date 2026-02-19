# Simplify Sweatshop

## Goal

Strip sweatshop back to its core purpose: worktree lifecycle management with
minimal friction. Remove the interactive close-shop mega-prompt, the env
snapshot/review cycle, and the perm snapshot-on-close flow. Reduce spinclass
to a symlink.

## Motivation

The close-shop flow tries to be a wizard for operations that are better as
discrete commands (`merge`, `clean`, `pull`). The env and perm review cycles
add noise to every session close for a benefit that's rarely wanted. Spinclass
duplicates 194 lines of main.go for zero functional difference.

## Design

### CloseShop becomes a status print

Replace the 7-option huh menu and multi-step git workflow with a one-line
status summary printed after zmx detaches.

Human-readable format:

```
eng/worktrees/lux/fix-hover: 3 commits ahead of master, clean
eng/worktrees/grit/add-tags: 1 commit ahead of master, dirty (2M 1?)
eng/worktrees/dodger/refactor: 0 commits ahead, clean (merged)
```

TAP format: `ok 1 - close eng/worktrees/lux/fix-hover # 3 ahead, clean`

No prompts, no git operations, no env review, no perm review.

#### Deleted code

- `chooseAction()`, `executeAction()`, `containsRemoveWorktree()`,
  `estimateSteps()` from `shop.go`
- `shop_test.go` (tests `estimateSteps`)
- Env review block in `CloseShop` (huh prompts, `DiffEnv`, `RouteEnvDecisions`)
- Perm review block in `CloseShop` (`RunReviewInteractive`, `CleanupSnapshot`)
- Action menu block in `CloseShop` (huh select, `executeAction` dispatch)
- `huh` import from `shop.go`

#### Kept code

- `perms review` standalone command
- `perms check`, `perms list`, `perms edit`
- `merge` command (explicit, user-initiated)

### Env snapshot/review removal

Delete `internal/sweatfile/env_review.go` entirely:

- `SnapshotEnv`, `DiffEnv`, `CleanupEnvSnapshot`, `parseEnvFile`,
  `RouteEnvDecisions`, `removeEnvKeys`
- `EnvDecision` type, `EnvPromoteRepo`/`EnvKeep`/`EnvDiscard` constants

Delete `internal/sweatfile/env_review_test.go`.

Remove `sweatfile.SnapshotEnv()` calls from `OpenExisting` and `OpenNew`.

Sweatfiles become purely create-time templates. The `.sweatshop-env` file is
still written during `sweatfile.Apply()` at worktree creation; no runtime
diffing or promotion flow.

### Perm snapshot removal from close flow

Remove from close flow only — standalone `perms review` command stays.

- Remove `SnapshotSettings()` calls from `OpenExisting` and `OpenNew`
- Remove `CleanupSnapshot()` call from `CloseShop`
- Remove `RunReviewInteractive()` call from `CloseShop`
- Remove `SnapshotSettings()` and `CleanupSnapshot()` functions from
  `internal/perms/cmd.go`
- Remove `--integrate-perms-on-close` flag from both main.go files
- Remove `integratePerms bool` parameter from `OpenExisting`, `OpenNew`,
  `CloseShop`

`perms review` stays as a standalone command. Without the automatic
snapshot-on-open, it will error if no snapshot exists. This is acceptable;
reworking `perms review` to snapshot on demand is a future TODO.

### Spinclass becomes a symlink

Delete `cmd/spinclass/main.go`. In `flake.nix`, create `spinclass` as:

- A `bin/spinclass` symlink pointing to `sweatshop`
- Shell completions with the name `spinclass` (copies of sweatshop's
  completions with the command name swapped)

Add `os.Args[0]` detection in `cmd/sweatshop/main.go` so help text and error
messages reflect whichever name was invoked:

```go
name := filepath.Base(os.Args[0])
rootCmd.Use = name
```

### Updated function signatures

```go
shop.OpenExisting(sweatshopPath, format, noAttach, claudeArgs)
shop.OpenNew(sweatshopPath, format, noAttach, claudeArgs)
shop.CloseShop(sweatshopPath, format)
```

## Scope summary

| Change | Effect |
|--------|--------|
| CloseShop to status print | ~250 lines deleted, ~20 added |
| Env review removed | `env_review.go` + tests deleted |
| Perm snapshot removed from close | Snapshot functions + flag deleted |
| Spinclass to symlink | `cmd/spinclass/main.go` deleted, flake.nix updated |

## Files deleted

- `cmd/spinclass/main.go`
- `internal/sweatfile/env_review.go`
- `internal/sweatfile/env_review_test.go`
- `internal/shop/shop_test.go`

## Out of scope

- Reworking `perms review` to snapshot on demand
- Purse-first migration
- Expanding permission injection on `open` (Tier 2)
- tap-dancer migration
