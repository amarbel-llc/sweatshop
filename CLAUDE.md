# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`z` is a shell-agnostic git worktree session manager that wraps `zmx` (a terminal multiplexer session manager). It manages the lifecycle of git worktrees: creating them in a convention-based directory structure, attaching to terminal sessions via zmx, and offering post-session workflows (rebase, merge, cleanup, push). Supports both local and remote (SSH) worktrees.

## Commands

```sh
just build    # nix build
just test     # bats tests via: nix develop --command bats tests/
just check    # shellcheck on all scripts in bin/
just fmt      # shfmt -w -i 2 -ci on all scripts in bin/
just run      # nix run . -- [args]
```

Run a single test file: `nix develop --command bats tests/test_parse_target.bats`

## Architecture

Three Bash scripts in `bin/`, packaged via Nix with `writeScriptBin` + `symlinkJoin`:

- **`z`** — Main entry point. Parses `[host:]path` targets, creates worktrees, attaches zmx sessions, and presents interactive post-session menus (via `gum choose`). All functions prefixed `z_`.
- **`z-merge`** — Run from inside a worktree. Merges branch into main with `--no-ff`, removes the worktree, and detaches from zmx.
- **`z-completions`** — Generates tab-separated completions by scanning `~/eng*/repos/` (local) and querying `Z_REMOTE_HOSTS` or `~/.config/z/remotes` (remote via SSH).

### Convention-based directory layout

All paths are relative to `$HOME` and follow: `<eng_area>/worktrees/<repo>/<branch>`. Repositories live at `<eng_area>/repos/<repo>`. The rcm-worktrees overlay copies dotfiles from `<eng_area>/rcm-worktrees/` into new worktrees as hidden files.

### Nix packaging

Follows the stable-first nixpkgs convention (`nixpkgs` = stable, `nixpkgs-master` = unstable). The devShell inherits from `friedenberg/eng?dir=devenvs/shell` and adds `just` and `gum`. Runtime dependency `gum` is bundled and PATH-wrapped.

## Testing

Uses bats (minimum 1.5.0) with test isolation via temporary HOME directories and mock PATH (`tests/common.bash`). Three test files cover target parsing, completions, and rcm overlay logic. Scripts are sourced (not executed) in tests to unit-test individual functions.
