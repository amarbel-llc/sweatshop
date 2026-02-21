# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`sweatshop` is a shell-agnostic git worktree session manager that wraps `zmx` (a terminal multiplexer session manager). It manages the lifecycle of git worktrees: creating them inside repositories at `<repo>/.worktrees/<branch>`, attaching to terminal sessions via zmx, and offering post-session workflows (merge, cleanup, pull/rebase).

Written in Go with cobra for CLI, lipgloss/table for styled output, huh for interactive prompts, and charmbracelet/log for structured logging.

## Commands

```sh
just build            # build all (gomod2nix + go build)
just build-go         # go build via nix develop
just build-gomod2nix  # regenerate gomod2nix.toml
just test             # run all tests (go + bats)
just test-go          # go unit tests
just test-bats        # bats integration tests
just codemod-fmt      # format all code
just codemod-fmt-go   # gofumpt -w .
just update-go        # go mod tidy + gomod2nix
just run-nix          # nix run . -- [args]
just clean            # remove build artifacts
```

Run a single test file: `nix develop --command bats tests/test_status.bats`

## Architecture

Single Go binary with cobra subcommands:

```
cmd/sweatshop/main.go              # cobra root + subcommand registration
internal/
  shop/shop.go                      # create/attach orchestration
  merge/merge.go                    # merge subcommand (--ff-only merge, worktree remove)
  clean/clean.go                    # clean subcommand (remove merged worktrees)
  pull/pull.go                      # pull subcommand (pull repos, rebase worktrees)
  completions/completions.go        # completions subcommand (PWD-relative scanning)
  status/status.go                  # status subcommand + lipgloss table rendering
  worktree/worktree.go              # shared: path resolution, worktree creation, scanning
  sweatfile/sweatfile.go            # sweatfile parsing, hierarchy loading, merging
  git/git.go                        # shared: git command execution helpers
  claude/claude.go                  # Claude Code workspace trust management
  perms/cmd.go                      # permission review subcommand
```

### Worktree directory layout

Worktrees live inside each repository at `<repo>/.worktrees/<branch>`. The `.worktrees` directory is added to `.git/info/exclude` on first worktree creation. All commands are PWD-relative: run from inside a repo to operate on that repo, or from a parent directory to scan child repos.

### Sweatfile hierarchy

Sweatfile configuration is loaded from multiple locations, merged top-down:
1. Global: `~/.config/sweatshop/sweatfile`
2. Parent directories walking down from `$HOME` to the repo
3. Repo: `<repo>/sweatfile`

Arrays use nil=inherit, empty=clear, non-empty=append semantics.

### Nix packaging

Uses `buildGoApplication` with gomod2nix. The Go devenv is inherited from `friedenberg/eng?dir=devenvs/go` which provides the gomod2nix overlay. Shell completions are installed separately via `runCommand`.

## Testing

- **Go unit tests**: `internal/worktree/`, `internal/sweatfile/`, `internal/status/`, `internal/completions/`, `internal/claude/` — test path resolution, sweatfile loading/merging, dirty status parsing, table rendering, completion generation, workspace trust
- **Bats integration tests**: `tests/` — test the compiled binary with isolated HOME directories and mock git repos (status, completions, sweatfile, clean, pull)

## Notes

- GPG signing is required for commits. If signing fails, ask user to unlock their agent rather than skipping signatures
- Module path is `github.com/amarbel-llc/sweatshop`
