# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`sweatshop` is a shell-agnostic git worktree session manager that wraps `zmx` (a terminal multiplexer session manager). It manages the lifecycle of git worktrees: creating them in a convention-based directory structure, attaching to terminal sessions via zmx, and offering post-session workflows (rebase, merge, cleanup, push). Supports both local and remote (SSH) worktrees.

Written in Go with cobra for CLI, lipgloss/table for styled output, huh for interactive prompts, and charmbracelet/log for structured logging.

## Commands

```sh
just build         # nix build
just build-go      # go build via nix develop
just build-gomod2nix # regenerate gomod2nix.toml
just test          # go unit tests via: nix develop --command go test ./...
just test-bats     # bats integration tests via: nix develop --command bats tests/
just fmt           # gofumpt -w .
just deps          # go mod tidy + gomod2nix
just run           # nix run . -- [args]
```

Run a single test file: `nix develop --command bats tests/test_status.bats`

## Architecture

Single Go binary with cobra subcommands:

```
cmd/sweatshop/main.go              # cobra root + subcommand registration
internal/
  attach/attach.go                  # attach subcommand (local, remote, post-zmx with huh prompts)
  merge/merge.go                    # merge subcommand (--no-ff merge, worktree remove, zmx detach)
  completions/completions.go        # completions subcommand (local + remote scanning)
  status/status.go                  # status subcommand + lipgloss table rendering
  worktree/worktree.go              # shared: path parsing, worktree creation, rcm overlay
  git/git.go                        # shared: git command execution helpers
```

### Convention-based directory layout

All paths are relative to `$HOME` and follow: `<eng_area>/worktrees/<repo>/<branch>`. Repositories live at `<eng_area>/repos/<repo>`. The rcm-worktrees overlay copies dotfiles from `<eng_area>/rcm-worktrees/` into new worktrees as hidden files.

### Nix packaging

Uses `buildGoApplication` with gomod2nix. The Go devenv is inherited from `friedenberg/eng?dir=devenvs/go` which provides the gomod2nix overlay. Shell completions are installed separately via `runCommand`.

## Testing

- **Go unit tests**: `internal/worktree/`, `internal/status/`, `internal/completions/` — test target parsing, path validation, dirty status parsing, table rendering, completion generation
- **Bats integration tests**: `tests/test_status.bats`, `tests/test_completions.bats` — test the compiled binary with isolated HOME directories and mock git repos

## Notes

- GPG signing is required for commits. If signing fails, ask user to unlock their agent rather than skipping signatures
- Module path is `github.com/amarbel-llc/sweatshop`
