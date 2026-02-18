# Update Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an `update` subcommand that pulls all repos and rebases all worktrees, with TAP-14 output and a `--dirty` flag.

**Architecture:** New `internal/update/` package following the `clean` package pattern. Two-phase execution: scan repos and pull, then scan worktrees and rebase. Uses existing `internal/tap` writer and `internal/git` helpers.

**Tech Stack:** Go, cobra, internal/tap, internal/git, bats for integration tests.

---

### Task 1: Add `Pull` and `Rebase` helpers to `internal/git/git.go`

**Files:**
- Modify: `internal/git/git.go`

**Step 1: Add `Pull` function**

Append to `internal/git/git.go`:

```go
func Pull(repoPath string) (string, error) {
	return Run(repoPath, "pull")
}
```

**Step 2: Add `Rebase` function**

Append to `internal/git/git.go`:

```go
func Rebase(repoPath, onto string) (string, error) {
	return Run(repoPath, "rebase", onto)
}
```

**Step 3: Commit**

```bash
git add internal/git/git.go
git commit -m "feat(git): add Pull and Rebase helpers"
```

---

### Task 2: Create `internal/update/update.go` with repo scanning and pulling

**Files:**
- Create: `internal/update/update.go`

**Step 1: Write `internal/update/update.go`**

```go
package update

import (
	"os"
	"path/filepath"

	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/tap"
)

type repoInfo struct {
	engArea  string
	name     string
	repoPath string
	dirty    bool
}

type worktreeInfo struct {
	engArea      string
	repo         string
	branch       string
	repoPath     string
	worktreePath string
	dirty        bool
}

func scanRepos(home string) []repoInfo {
	var repos []repoInfo

	pattern := filepath.Join(home, "eng*", "repos")
	matches, _ := filepath.Glob(pattern)

	for _, reposDir := range matches {
		engArea := filepath.Base(filepath.Dir(reposDir))
		entries, err := os.ReadDir(reposDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(reposDir, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}
			porcelain := git.StatusPorcelain(repoPath)
			repos = append(repos, repoInfo{
				engArea:  engArea,
				name:     entry.Name(),
				repoPath: repoPath,
				dirty:    porcelain != "",
			})
		}
	}

	return repos
}

func scanWorktrees(home string, repos []repoInfo) []worktreeInfo {
	var worktrees []worktreeInfo

	for _, repo := range repos {
		wtDir := filepath.Join(home, repo.engArea, "worktrees", repo.name)
		entries, err := os.ReadDir(wtDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			wtPath := filepath.Join(wtDir, entry.Name())
			porcelain := git.StatusPorcelain(wtPath)
			worktrees = append(worktrees, worktreeInfo{
				engArea:      repo.engArea,
				repo:         repo.name,
				branch:       entry.Name(),
				repoPath:     repo.repoPath,
				worktreePath: wtPath,
				dirty:        porcelain != "",
			})
		}
	}

	return worktrees
}

func Run(home string, dirty bool) error {
	tw := tap.NewWriter(os.Stdout)

	repos := scanRepos(home)
	worktrees := scanWorktrees(home, repos)

	if len(repos) == 0 && len(worktrees) == 0 {
		tw.Skip("update", "no repos found")
		tw.Plan()
		return nil
	}

	for _, repo := range repos {
		label := repo.engArea + "/repos/" + repo.name

		if repo.dirty && !dirty {
			tw.Skip("pull "+label, "dirty")
			continue
		}

		_, err := git.Pull(repo.repoPath)
		if err != nil {
			tw.NotOk("pull "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			continue
		}
		tw.Ok("pull " + label)
	}

	for _, wt := range worktrees {
		label := wt.engArea + "/worktrees/" + wt.repo + "/" + wt.branch

		if wt.dirty && !dirty {
			tw.Skip("rebase "+label, "dirty")
			continue
		}

		defaultBranch, err := git.BranchCurrent(wt.repoPath)
		if err != nil || defaultBranch == "" {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  "could not determine default branch",
				"severity": "fail",
			})
			continue
		}

		_, err = git.Rebase(wt.worktreePath, defaultBranch)
		if err != nil {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			continue
		}
		tw.Ok("rebase " + label)
	}

	tw.Plan()
	return nil
}
```

**Step 2: Verify it compiles**

Run: `nix develop --command go build ./internal/update/`
Expected: no errors

**Step 3: Commit**

```bash
git add internal/update/update.go
git commit -m "feat(update): add update package with pull and rebase phases"
```

---

### Task 3: Register `updateCmd` in cobra

**Files:**
- Modify: `cmd/sweatshop/main.go`
- Modify: `cmd/spinclass/main.go`

**Step 1: Add import and command to `cmd/sweatshop/main.go`**

Add to imports:

```go
"github.com/amarbel-llc/sweatshop/internal/update"
```

Add the command variable after `cleanInteractive`:

```go
var updateDirty bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Pull repos and rebase worktrees",
	Long:  `Pull all clean repos, then rebase all clean worktrees onto their repo's default branch. Use -d to include dirty repos and worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return update.Run(home, updateDirty)
	},
}
```

Add to `init()`:

```go
updateCmd.Flags().BoolVarP(&updateDirty, "dirty", "d", false, "include dirty repos and worktrees")
rootCmd.AddCommand(updateCmd)
```

**Step 2: Apply identical changes to `cmd/spinclass/main.go`**

Same import, command variable, and `init()` additions.

**Step 3: Verify it compiles**

Run: `nix develop --command go build ./cmd/sweatshop/ && nix develop --command go build ./cmd/spinclass/`
Expected: no errors

**Step 4: Commit**

```bash
git add cmd/sweatshop/main.go cmd/spinclass/main.go
git commit -m "feat(cli): register update subcommand in sweatshop and spinclass"
```

---

### Task 4: Write bats integration tests

**Files:**
- Create: `tests/test_update.bats`

**Step 1: Write `tests/test_update.bats`**

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path
}

create_repo_with_remote() {
  local repo_path="$1"
  local bare_path="$2"

  # Create a bare repo as the "remote"
  mkdir -p "$bare_path"
  git -C "$bare_path" init -q --bare

  # Clone it to create the working repo
  git clone -q "$bare_path" "$repo_path"
  git -C "$repo_path" commit --allow-empty -m "init" -q
  git -C "$repo_path" push -q
}

push_remote_commit() {
  local bare_path="$1"
  local tmp_clone="$BATS_TEST_TMPDIR/tmp-clone-$$"

  git clone -q "$bare_path" "$tmp_clone"
  git -C "$tmp_clone" commit --allow-empty -m "remote update" -q
  git -C "$tmp_clone" push -q
  rm -rf "$tmp_clone"
}

create_worktree() {
  local repo_path="$1"
  local branch="$2"
  local worktree_path="$3"

  mkdir -p "$(dirname "$worktree_path")"
  git -C "$repo_path" worktree add -q "$worktree_path" -b "$branch"
  git -C "$worktree_path" commit --allow-empty -m "worktree commit" -q
}

function update_pulls_clean_repos { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  push_remote_commit "$bare"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"TAP version 14"* ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
}

function update_skips_dirty_repos_without_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP dirty"* ]]
}

function update_includes_dirty_repos_with_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop update --dirty
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
  [[ "$output" != *"# SKIP"* ]]
}

function update_rebases_clean_worktrees { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"rebase eng/worktrees/myrepo/feature-x"* ]]
}

function update_skips_dirty_worktrees_without_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"
  echo "uncommitted" > "$HOME/eng/worktrees/myrepo/feature-x/dirty.txt"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"rebase"*"# SKIP dirty"* ]]
}

function update_includes_dirty_worktrees_with_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"
  echo "uncommitted" > "$HOME/eng/worktrees/myrepo/feature-x/dirty.txt"

  run sweatshop update -d
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"rebase eng/worktrees/myrepo/feature-x"* ]]
}

function update_reports_plan_line { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1..2"* ]]
}

function update_shows_skip_when_no_repos { # @test
  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP no repos found"* ]]
}

function update_works_across_eng_areas { # @test
  local bare_a="$BATS_TEST_TMPDIR/bare/repo-a.git"
  local bare_b="$BATS_TEST_TMPDIR/bare/repo-b.git"
  create_repo_with_remote "$HOME/eng/repos/repo-a" "$bare_a"
  create_repo_with_remote "$HOME/eng2/repos/repo-b" "$bare_b"

  run sweatshop update
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"pull eng/repos/repo-a"* ]]
  [[ "$output" == *"pull eng2/repos/repo-b"* ]]
  [[ "$output" == *"1..2"* ]]
}

function update_short_flag_d_works { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop update -d
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
}
```

**Step 2: Build and run tests**

Run: `just build && nix develop --command bats tests/test_update.bats`
Expected: all tests pass

**Step 3: Commit**

```bash
git add tests/test_update.bats
git commit -m "test(update): add bats integration tests for update command"
```

---

### Task 5: Update gomod2nix and verify nix build

**Step 1: Regenerate gomod2nix.toml**

Run: `just deps`

**Step 2: Verify nix build**

Run: `just build`
Expected: builds successfully

**Step 3: Run full test suite**

Run: `just test && just test-bats`
Expected: all tests pass

**Step 4: Commit if gomod2nix.toml changed**

```bash
git add gomod2nix.toml go.mod go.sum
git commit -m "chore: update go dependencies"
```
