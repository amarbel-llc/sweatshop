# Simplify Sweatshop Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Strip CloseShop to a status print, remove env/perm review-on-close, reduce spinclass to a symlink.

**Architecture:** Four independent deletions (env review, perm snapshot, close-shop menu, spinclass binary) followed by a new lightweight CloseShop that prints a one-line status. Each task is a self-contained commit.

**Tech Stack:** Go 1.23, cobra, charmbracelet/log, nix flake with buildGoApplication

---

### Task 1: Delete env snapshot/review system

Remove the entire env snapshot/diff/review cycle. Sweatfiles become create-time-only templates.

**Files:**
- Delete: `internal/sweatfile/env_review.go`
- Delete: `internal/sweatfile/env_review_test.go`
- Modify: `internal/shop/shop.go` (remove env snapshot and review code)

**Step 1: Delete env_review.go and env_review_test.go**

Delete the files:
- `internal/sweatfile/env_review.go`
- `internal/sweatfile/env_review_test.go`

**Step 2: Remove env snapshot call from OpenExisting**

In `internal/shop/shop.go`, function `OpenExisting` (line 33), remove:

```go
sweatfile.SnapshotEnv(worktreePath)
```

(line 48)

**Step 3: Remove env snapshot call from OpenNew**

In `internal/shop/shop.go`, function `OpenNew` (line 67), remove:

```go
sweatfile.SnapshotEnv(worktreePath)
```

(line 96)

**Step 4: Remove env review block from CloseShop**

In `internal/shop/shop.go`, function `CloseShop` (line 124), remove the entire env review block (lines 156-201) — from `// Review env changes` through the end of the `if routeErr` block and the `sweatfile.CleanupEnvSnapshot(worktreePath)` call (line 201).

**Step 5: Remove the sweatfile import from shop.go**

After removing the env calls, the `"github.com/amarbel-llc/sweatshop/internal/sweatfile"` import in `shop.go` becomes unused. Remove it.

**Step 6: Run unit tests**

Run: `nix develop --command go test ./...`
Expected: All tests pass. The `env_review_test.go` tests are gone. No other test references these functions.

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove env snapshot/review system

Sweatfiles are create-time templates only. Remove the snapshot-on-open,
diff-on-close, and interactive promotion flow for env vars.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Remove perm snapshot from close flow

Remove `SnapshotSettings`/`CleanupSnapshot` calls from open/close, the `--integrate-perms-on-close` flag, and the `integratePerms` parameter. Keep `perms review` as a standalone command.

**Files:**
- Modify: `internal/shop/shop.go` (remove perm snapshot calls, `integratePerms` param, perm review block)
- Modify: `internal/perms/cmd.go` (delete `SnapshotSettings` and `CleanupSnapshot` functions)
- Modify: `cmd/sweatshop/main.go` (remove flag and parameter)
- Modify: `cmd/spinclass/main.go` (remove flag and parameter)

**Step 1: Remove SnapshotSettings and CleanupSnapshot from perms/cmd.go**

In `internal/perms/cmd.go`, delete:
- `SnapshotSettings` function (lines 238-255)
- `CleanupSnapshot` function (lines 257-260)

Also remove the `"os"` import if it becomes unused (it's still used by `newEditCmd` and `newListCmd`, so it stays).

**Step 2: Remove integratePerms from shop.go function signatures**

In `internal/shop/shop.go`:

Change `OpenExisting` signature (line 33) from:
```go
func OpenExisting(sweatshopPath, format string, noAttach, integratePerms bool, claudeArgs []string) error {
```
to:
```go
func OpenExisting(sweatshopPath, format string, noAttach bool, claudeArgs []string) error {
```

Remove the perm snapshot block (lines 45-47):
```go
if integratePerms {
    perms.SnapshotSettings(worktreePath)
}
```

Change `OpenNew` signature (line 67) from:
```go
func OpenNew(sweatshopPath, format string, noAttach, integratePerms bool, claudeArgs []string) error {
```
to:
```go
func OpenNew(sweatshopPath, format string, noAttach bool, claudeArgs []string) error {
```

Remove the perm snapshot block (lines 93-95):
```go
if integratePerms {
    perms.SnapshotSettings(worktreePath)
}
```

Change `CloseShop` signature (line 124) from:
```go
func CloseShop(sweatshopPath, format string, integratePerms bool) error {
```
to:
```go
func CloseShop(sweatshopPath, format string) error {
```

Remove the perm review block (lines 148-153):
```go
if integratePerms {
    if reviewErr := perms.RunReviewInteractive(sweatshopPath); reviewErr != nil {
        log.Warn("permission review skipped", "error", reviewErr)
    }
    perms.CleanupSnapshot(worktreePath)
}
```

Update the `CloseShop` call in `OpenExisting` (line 64) from:
```go
return CloseShop(sweatshopPath, format, integratePerms)
```
to:
```go
return CloseShop(sweatshopPath, format)
```

Update the `CloseShop` call in `OpenNew` (line 121) from:
```go
return CloseShop(sweatshopPath, format, integratePerms)
```
to:
```go
return CloseShop(sweatshopPath, format)
```

Update the `OpenExisting` call inside the "Abort" action (line 238) from:
```go
return OpenExisting(sweatshopPath, format, false, integratePerms, nil)
```
to:
```go
return OpenExisting(sweatshopPath, format, false, nil)
```

Remove the `"github.com/amarbel-llc/sweatshop/internal/perms"` import from `shop.go` (it's no longer referenced after removing the `RunReviewInteractive` and `SnapshotSettings`/`CleanupSnapshot` calls).

**Step 3: Remove --integrate-perms-on-close flag from cmd/sweatshop/main.go**

In `cmd/sweatshop/main.go`:

Remove the `openIntegratePerms` variable from the var block (line 119).

Remove the flag registration (line 177):
```go
openCmd.Flags().BoolVar(&openIntegratePerms, "integrate-perms-on-close", false, "review and integrate Claude permission changes on close")
```

Update all `shop.OpenExisting` and `shop.OpenNew` calls to remove the `openIntegratePerms` argument:
- Line 58: `shop.OpenExisting(sweatshopPath, format, openNoAttach, claudeArgs)`
- Line 60: `shop.OpenNew(sweatshopPath, format, openNoAttach, claudeArgs)`
- Line 71: `shop.OpenExisting(target.Path, format, openNoAttach, claudeArgs)`
- Line 74: `shop.OpenNew(target.Path, format, openNoAttach, claudeArgs)`

**Step 4: Remove --integrate-perms-on-close flag from cmd/spinclass/main.go**

Same changes as Step 3 but in `cmd/spinclass/main.go`.

**Step 5: Run unit tests**

Run: `nix develop --command go test ./...`
Expected: All tests pass.

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove perm snapshot from close flow

Remove SnapshotSettings/CleanupSnapshot, the --integrate-perms-on-close
flag, and the integratePerms parameter. perms review stays as a
standalone command.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Replace CloseShop with status print

Gut the interactive close-shop flow (huh menu, multi-step git operations) and replace it with a one-line status print.

**Files:**
- Modify: `internal/shop/shop.go` (rewrite `CloseShop`, delete helper functions)
- Delete: `internal/shop/shop_test.go`

**Step 1: Delete shop_test.go**

Delete `internal/shop/shop_test.go` (tests `estimateSteps` which will no longer exist).

**Step 2: Rewrite CloseShop**

Replace the entire `CloseShop` function and delete `chooseAction`, `runGit`, `tapStep`, `executeAction`, `containsRemoveWorktree`, `estimateSteps`.

New `CloseShop`:

```go
func CloseShop(sweatshopPath, format string) error {
	comp, err := worktree.ParsePath(sweatshopPath)
	if err != nil {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	repoPath := worktree.RepoPath(home, comp)
	worktreePath := worktree.WorktreePath(home, sweatshopPath)

	defaultBranch, err := git.BranchCurrent(repoPath)
	if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	commitsAhead := git.CommitsAhead(worktreePath, defaultBranch, comp.Worktree)
	worktreeStatus := git.StatusPorcelain(worktreePath)

	desc := statusDescription(comp.Worktree, defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.PlanAhead(1)
		tw.Ok("close " + comp.Worktree + " # " + desc)
	} else {
		log.Info(desc, "worktree", sweatshopPath)
	}

	return nil
}

func statusDescription(worktree, defaultBranch string, commitsAhead int, porcelain string) string {
	var parts []string

	if commitsAhead == 1 {
		parts = append(parts, fmt.Sprintf("1 commit ahead of %s", defaultBranch))
	} else {
		parts = append(parts, fmt.Sprintf("%d commits ahead of %s", commitsAhead, defaultBranch))
	}

	if porcelain == "" {
		parts = append(parts, "clean")
	} else {
		parts = append(parts, "dirty")
	}

	if commitsAhead == 0 && porcelain == "" {
		parts = append(parts, "(merged)")
	}

	return strings.Join(parts, ", ")
}
```

**Step 3: Clean up imports in shop.go**

After the rewrite, remove unused imports. The following should be removed:
- `"path/filepath"` (no longer used after env review removal)
- `"github.com/charmbracelet/huh"` (no more interactive prompts)
- `"github.com/charmbracelet/lipgloss"` (no more styled code rendering)
- `"github.com/amarbel-llc/sweatshop/internal/perms"` (already removed in Task 2)
- `"github.com/amarbel-llc/sweatshop/internal/sweatfile"` (already removed in Task 1)

Also delete the `styleCode` variable (line 22) since lipgloss is no longer used.

Keep:
- `"fmt"`, `"os"`, `"os/exec"`, `"strings"`
- `"github.com/charmbracelet/log"`
- `"github.com/amarbel-llc/sweatshop/internal/flake"`
- `"github.com/amarbel-llc/sweatshop/internal/git"`
- `"github.com/amarbel-llc/sweatshop/internal/tap"`
- `"github.com/amarbel-llc/sweatshop/internal/worktree"`

**Step 4: Write a test for statusDescription**

Create `internal/shop/shop_test.go`:

```go
package shop

import "testing"

func TestStatusDescription(t *testing.T) {
	tests := []struct {
		name          string
		worktree      string
		defaultBranch string
		commitsAhead  int
		porcelain     string
		want          string
	}{
		{
			name:          "ahead and clean",
			worktree:      "feature-x",
			defaultBranch: "master",
			commitsAhead:  3,
			porcelain:     "",
			want:          "3 commits ahead of master, clean",
		},
		{
			name:          "one commit ahead",
			worktree:      "feature-x",
			defaultBranch: "master",
			commitsAhead:  1,
			porcelain:     "",
			want:          "1 commit ahead of master, clean",
		},
		{
			name:          "ahead and dirty",
			worktree:      "fix-bug",
			defaultBranch: "main",
			commitsAhead:  2,
			porcelain:     "M file.go\n",
			want:          "2 commits ahead of main, dirty",
		},
		{
			name:          "merged",
			worktree:      "done",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "",
			want:          "0 commits ahead of master, clean, (merged)",
		},
		{
			name:          "zero ahead but dirty",
			worktree:      "wip",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "?? untracked\n",
			want:          "0 commits ahead of master, dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusDescription(tt.worktree, tt.defaultBranch, tt.commitsAhead, tt.porcelain)
			if got != tt.want {
				t.Errorf("statusDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 5: Run tests**

Run: `nix develop --command go test ./...`
Expected: All tests pass including the new `statusDescription` tests.

**Step 6: Run bats integration tests**

Run: `nix build && nix develop --command bats --tap tests/`
Expected: All bats tests pass. The sweatfile tests use `--format tap` and mock zmx to exit immediately, so the new lightweight CloseShop will run and print a status line without blocking.

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor: replace CloseShop with status print

Remove the 7-option huh menu and multi-step git workflow. After zmx
detaches, print a one-line status (commits ahead, dirty state) and
exit. Use explicit commands (merge, clean, pull) for git operations.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Reduce spinclass to a symlink

Delete the duplicate `cmd/spinclass/main.go`, add `os.Args[0]` name detection in sweatshop's main.go, and update `flake.nix` to create spinclass as a symlink.

**Files:**
- Delete: `cmd/spinclass/main.go`
- Modify: `cmd/sweatshop/main.go` (add binary name detection)
- Modify: `flake.nix` (spinclass becomes symlink + completions)

**Step 1: Delete cmd/spinclass/main.go**

Delete the file `cmd/spinclass/main.go`.

**Step 2: Add os.Args[0] name detection to cmd/sweatshop/main.go**

In `cmd/sweatshop/main.go`, add `"path/filepath"` to imports and update `main()`:

```go
func main() {
	rootCmd.Use = filepath.Base(os.Args[0])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 3: Update flake.nix**

Replace the `spinclass` `buildGoApplication` with a `runCommand` that creates a symlink:

```nix
spinclass = pkgs.runCommand "spinclass" { } ''
  mkdir -p $out/bin
  ln -s ${sweatshop}/bin/sweatshop $out/bin/spinclass
'';
```

Update the `packages.spinclass` output to use the new spinclass + completions:

```nix
spinclass = pkgs.symlinkJoin {
  name = "spinclass";
  paths = [
    spinclass
    shellCompletions
  ];
};
```

Remove `subPackages = [ "cmd/spinclass" ];` from the old spinclass build.

Update `apps.spinclass` to point at the symlinked binary:

```nix
spinclass = {
  type = "app";
  program = "${spinclass}/bin/spinclass";
};
```

The completions for spinclass already exist as separate files (`completions/spinclass.bash-completion`, `completions/spinclass.fish`) and reference `spinclass` by name (they call `spinclass completions`). Since the symlink makes `spinclass` resolve to the same binary, these completions continue to work. No changes needed to completion files.

**Step 4: Build and test**

Run: `nix build`
Expected: Build succeeds.

Run: `nix build .#spinclass`
Expected: Build succeeds. `result/bin/spinclass` is a symlink to `sweatshop`.

Run: `nix run .#spinclass -- --help`
Expected: Help text says `spinclass` (via `os.Args[0]` detection).

Run: `nix develop --command go test ./...`
Expected: All tests pass.

Run: `nix build && nix develop --command bats --tap tests/`
Expected: All bats tests pass.

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor: reduce spinclass to a symlink

Delete cmd/spinclass/main.go. Create spinclass as a symlink to
sweatshop in the nix package. Add os.Args[0] detection so help
text reflects the invoked binary name.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Final verification

**Step 1: Run full test suite**

Run: `nix develop --command go test ./...`
Expected: All unit tests pass.

**Step 2: Run bats tests**

Run: `nix build && nix develop --command bats --tap tests/`
Expected: All integration tests pass.

**Step 3: Nix flake check**

Run: `nix flake check`
Expected: No errors.

**Step 4: Format check**

Run: `nix develop --command gofumpt -l .`
Expected: No output (all files formatted).
