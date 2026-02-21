# Worktrees-In-Repo Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move worktree storage from `~/<eng_area>/worktrees/<repo>/<branch>` into `<repo>/.worktrees/<branch>`, drop convention path syntax and SSH support, make all scanning PWD-relative.

**Architecture:** Incremental refactor of existing packages. Replace convention path parsing with repo-detection + `.worktrees/` resolution. Replace `~/eng*/repos` glob scanning with PWD-relative scanning. Remove remote/SSH code paths entirely.

**Tech Stack:** Go, cobra, lipgloss, bats for integration tests.

**Design doc:** `docs/plans/2026-02-21-worktrees-in-repo-design.md`

---

### Task 1: Rewrite `worktree` package path resolution

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write failing tests for new `ResolvePath`**

Replace all tests in `worktree_test.go` with tests for the new behavior. Remove
`TestParseTarget*`, `TestParsePath*`, `TestShopKey`, `TestResolvePathConvention*`,
`TestResolvePathArbitrary*`, `TestFindEngAreaDir*`.

Add these new tests:

```go
package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathBranchName(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)

	rp, err := ResolvePath(home, repoPath, "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.AbsPath != filepath.Join(repoPath, ".worktrees", "feature-x") {
		t.Errorf("unexpected AbsPath: %q", rp.AbsPath)
	}
	if rp.RepoPath != repoPath {
		t.Errorf("unexpected RepoPath: %q", rp.RepoPath)
	}
	if rp.SessionKey != "myrepo/feature-x" {
		t.Errorf("unexpected SessionKey: %q", rp.SessionKey)
	}
	if rp.Branch != "feature-x" {
		t.Errorf("unexpected Branch: %q", rp.Branch)
	}
}

func TestResolvePathRelativePath(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)

	rp, err := ResolvePath(home, repoPath, ".worktrees/feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.AbsPath != filepath.Join(repoPath, ".worktrees", "feature-x") {
		t.Errorf("unexpected AbsPath: %q", rp.AbsPath)
	}
	if rp.RepoPath != repoPath {
		t.Errorf("unexpected RepoPath: %q", rp.RepoPath)
	}
	if rp.Branch != "feature-x" {
		t.Errorf("unexpected Branch: %q", rp.Branch)
	}
}

func TestResolvePathAbsolutePath(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)
	absWorktree := filepath.Join(repoPath, ".worktrees", "feature-x")

	rp, err := ResolvePath(home, repoPath, absWorktree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.AbsPath != absWorktree {
		t.Errorf("unexpected AbsPath: %q", rp.AbsPath)
	}
	if rp.SessionKey != "myrepo/feature-x" {
		t.Errorf("unexpected SessionKey: %q", rp.SessionKey)
	}
}

func TestResolvePathSessionKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)

	rp, err := ResolvePath(home, repoPath, "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.SessionKey != "myrepo/feature-x" {
		t.Errorf("expected session key myrepo/feature-x, got %q", rp.SessionKey)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test ./internal/worktree/ -v`
Expected: FAIL — signature mismatch, `ResolvePath` takes different args.

**Step 3: Rewrite `worktree.go`**

Replace the contents of `internal/worktree/worktree.go` with:

```go
package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/sweatshop/internal/claude"
	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/sweatfile"
)

const WorktreesDir = ".worktrees"

type ResolvedPath struct {
	AbsPath    string // absolute filesystem path to the worktree
	RepoPath   string // absolute path to the parent git repo
	SessionKey string // key for zmx/executor sessions (repo/branch)
	Branch     string // branch name
}

// ResolvePath resolves a target argument into a ResolvedPath.
//
// repoPath is the absolute path to the git repo (detected from PWD by caller).
// target is either:
//   - a bare branch name (no "/" or ".") → <repo>/.worktrees/<branch>
//   - a relative path containing "/" or "." → resolved relative to repoPath
//   - an absolute path → used directly
func ResolvePath(home, repoPath, target string) (ResolvedPath, error) {
	var absPath string

	if filepath.IsAbs(target) {
		absPath = filepath.Clean(target)
	} else if strings.Contains(target, "/") || strings.Contains(target, ".") {
		absPath = filepath.Clean(filepath.Join(repoPath, target))
	} else {
		// Bare branch name
		absPath = filepath.Join(repoPath, WorktreesDir, target)
	}

	branch := filepath.Base(absPath)
	repoDirName := filepath.Base(repoPath)
	sessionKey := repoDirName + "/" + branch

	return ResolvedPath{
		AbsPath:    absPath,
		RepoPath:   repoPath,
		SessionKey: sessionKey,
		Branch:     branch,
	}, nil
}

// DetectRepo walks up from dir looking for a .git directory (that is a
// directory, not a file — files indicate worktrees). Returns the absolute
// path to the repo root.
func DetectRepo(dir string) (string, error) {
	dir = filepath.Clean(dir)
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository")
		}
		dir = parent
	}
}

func Create(repoPath, worktreePath string) (sweatfile.LoadResult, error) {
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("creating worktree directory: %w", err)
	}
	if err := git.RunPassthrough(repoPath, "worktree", "add", worktreePath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("git worktree add: %w", err)
	}

	// Add .worktrees to .git/info/exclude if not already there
	if err := excludeWorktreesDir(repoPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("adding .worktrees to git exclude: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("getting home directory: %w", err)
	}

	result, err := sweatfile.LoadHierarchy(home, repoPath)
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("loading sweatfile: %w", err)
	}
	if err := sweatfile.Apply(worktreePath, result.Merged); err != nil {
		return sweatfile.LoadResult{}, err
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	if err := claude.TrustWorkspace(claudeJSONPath, worktreePath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("trusting workspace in claude: %w", err)
	}

	return result, nil
}

func excludeWorktreesDir(repoPath string) error {
	excludePath := filepath.Join(repoPath, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}

	// Check if .worktrees is already excluded
	if data, err := os.ReadFile(excludePath); err == nil {
		if strings.Contains(string(data), WorktreesDir) {
			return nil
		}
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, WorktreesDir)
	return err
}

func IsWorktree(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (rp *ResolvedPath) FillBranchFromGit() error {
	branch, err := git.BranchCurrent(rp.AbsPath)
	if err != nil {
		return err
	}
	rp.Branch = branch
	return nil
}

// ScanRepos scans startDir for git repos containing .worktrees/ directories.
// If startDir is itself a repo, returns just that repo.
// Otherwise scans immediate children for repos.
func ScanRepos(startDir string) []string {
	// Check if startDir is a repo
	gitPath := filepath.Join(startDir, ".git")
	if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
		wtDir := filepath.Join(startDir, WorktreesDir)
		if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
			return []string{startDir}
		}
		return nil
	}

	// Scan children
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childPath := filepath.Join(startDir, entry.Name())
		gitPath := filepath.Join(childPath, ".git")
		if info, err := os.Stat(gitPath); err != nil || !info.IsDir() {
			continue
		}
		wtDir := filepath.Join(childPath, WorktreesDir)
		if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
			repos = append(repos, childPath)
		}
	}
	return repos
}

// ListWorktrees returns absolute paths to all worktrees in a repo's .worktrees/ dir.
func ListWorktrees(repoPath string) []string {
	wtDir := filepath.Join(repoPath, WorktreesDir)
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		return nil
	}
	var worktrees []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(wtDir, entry.Name())
		if IsWorktree(wtPath) {
			worktrees = append(worktrees, wtPath)
		}
	}
	return worktrees
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./internal/worktree/ -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "refactor: rewrite worktree package for .worktrees/ layout"
```

---

### Task 2: Rewrite sweatfile loading hierarchy

**Files:**
- Modify: `internal/sweatfile/sweatfile.go`
- Test: (add tests in same file or `sweatfile_test.go`)

**Step 1: Write failing test for `LoadHierarchy`**

Create `internal/sweatfile/sweatfile_test.go`:

```go
package sweatfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHierarchyGlobalOnly(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "sweatshop")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "sweatfile"), []byte(`git_excludes = [".direnv/"]`+"\n"), 0o644)

	repoDir := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Merged.GitExcludes) != 1 || result.Merged.GitExcludes[0] != ".direnv/" {
		t.Errorf("unexpected excludes: %v", result.Merged.GitExcludes)
	}
}

func TestLoadHierarchyGlobalAndRepo(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "sweatshop")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "sweatfile"), []byte(`git_excludes = [".direnv/"]`+"\n"), 0o644)

	repoDir := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "sweatfile"), []byte(`git_excludes = [".envrc"]`+"\n"), 0o644)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Merged.GitExcludes) != 2 {
		t.Errorf("expected 2 excludes, got %v", result.Merged.GitExcludes)
	}
}

func TestLoadHierarchyParentDir(t *testing.T) {
	home := t.TempDir()
	parentDir := filepath.Join(home, "eng")
	os.MkdirAll(parentDir, 0o755)
	os.WriteFile(filepath.Join(parentDir, "sweatfile"), []byte(`git_excludes = [".parent/"]`+"\n"), 0o644)

	repoDir := filepath.Join(parentDir, "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)
	os.WriteFile(filepath.Join(repoDir, "sweatfile"), []byte(`git_excludes = [".repo/"]`+"\n"), 0o644)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// global (none) + parent (.parent/) + repo (.repo/)
	if len(result.Merged.GitExcludes) != 2 {
		t.Errorf("expected 2 excludes, got %v", result.Merged.GitExcludes)
	}
}

func TestLoadHierarchyNoSweatfiles(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Merged.GitExcludes) != 0 {
		t.Errorf("expected no excludes, got %v", result.Merged.GitExcludes)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test ./internal/sweatfile/ -v`
Expected: FAIL — `LoadHierarchy` undefined.

**Step 3: Add `LoadHierarchy` function to `sweatfile.go`**

Add after `LoadSingle`:

```go
// LoadHierarchy loads and merges sweatfiles in order:
// 1. ~/.config/sweatshop/sweatfile (global)
// 2. Parent directories from home down to repo dir
// 3. <repo>/sweatfile
func LoadHierarchy(home, repoDir string) (LoadResult, error) {
	var sources []LoadSource
	merged := Sweatfile{}

	// 1. Global config
	globalPath := filepath.Join(home, ".config", "sweatshop", "sweatfile")
	globalSf, err := Load(globalPath)
	if err != nil {
		return LoadResult{}, err
	}
	_, globalFound := fileExists(globalPath)
	sources = append(sources, LoadSource{Path: globalPath, Found: globalFound, File: globalSf})
	merged = Merge(merged, globalSf)

	// 2. Parent directories between home and repo dir
	rel, err := filepath.Rel(home, repoDir)
	if err == nil && !strings.HasPrefix(rel, "..") {
		parts := strings.Split(rel, string(filepath.Separator))
		// Walk from first component down to (but not including) the last (repo itself)
		for i := 1; i < len(parts); i++ {
			dirPath := filepath.Join(home, filepath.Join(parts[:i]...))
			sfPath := filepath.Join(dirPath, "sweatfile")
			sf, err := Load(sfPath)
			if err != nil {
				return LoadResult{}, err
			}
			_, found := fileExists(sfPath)
			sources = append(sources, LoadSource{Path: sfPath, Found: found, File: sf})
			merged = Merge(merged, sf)
		}
	}

	// 3. Repo sweatfile
	repoSfPath := filepath.Join(repoDir, "sweatfile")
	repoSf, err := Load(repoSfPath)
	if err != nil {
		return LoadResult{}, err
	}
	_, repoFound := fileExists(repoSfPath)
	sources = append(sources, LoadSource{Path: repoSfPath, Found: repoFound, File: repoSf})
	merged = Merge(merged, repoSf)

	return LoadResult{
		Sources: sources,
		Merged:  merged,
	}, nil
}
```

Add `"strings"` to the import block.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./internal/sweatfile/ -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/sweatfile/sweatfile.go internal/sweatfile/sweatfile_test.go
git commit -m "feat: add LoadHierarchy for global/parent/repo sweatfile merging"
```

---

### Task 3: Update `shop` package and `cmd/sweatshop/main.go`

**Files:**
- Modify: `internal/shop/shop.go`
- Modify: `cmd/sweatshop/main.go`

**Step 1: Update `shop.Create` to use new `worktree.Create` signature**

In `internal/shop/shop.go`:
- Remove `OpenRemote` function entirely
- Update `Create` to call `worktree.Create(rp.RepoPath, rp.AbsPath)` (2 args instead of 3)
- Remove unused `sweatfile` import if it was only used for `LoadResult` logging (keep if `logSweatfileResult` still used)

```go
func Create(rp worktree.ResolvedPath, verbose bool) error {
	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}
	return os.Chdir(rp.AbsPath)
}
```

**Step 2: Update `cmd/sweatshop/main.go`**

Changes:
- Remove `createRepo` flag variable and `--repo` flag from create and attach commands
- `createCmd.RunE`: detect repo from PWD via `worktree.DetectRepo`, call `worktree.ResolvePath(home, repoPath, args[0])`
- `attachCmd.RunE`: remove `ParseTarget` / remote detection, detect repo from PWD, resolve path
- `completionsCmd.RunE`: call `completions.Local(cwd, os.Stdout)` instead of `completions.Local(home, os.Stdout)`, remove `completions.Remote`
- `statusCmd.RunE`: use PWD instead of home
- `cleanCmd.RunE`: use PWD instead of home
- `pullCmd.RunE`: use PWD instead of home
- Remove `"github.com/amarbel-llc/sweatshop/internal/perms"` import and `perms.NewPermsCmd()` if it depends on removed types (check first — if it's independent, keep it)

Update `createCmd`:
```go
var createCmd = &cobra.Command{
	Use:   "create <branch-or-path>",
	Short: "Create a worktree without attaching",
	Long:  `Create a new worktree and apply sweatfile settings. Does not start a session. Argument can be a branch name (creates .worktrees/<branch> in current repo), a relative path, or an absolute path.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return fmt.Errorf("must be run from inside a git repository: %w", err)
		}
		rp, err := worktree.ResolvePath(home, repoPath, args[0])
		if err != nil {
			return err
		}
		return shop.Create(rp, createVerbose)
	},
}
```

Update `attachCmd`:
```go
var attachCmd = &cobra.Command{
	Use:     "attach <branch-or-path> [claude args...]",
	Aliases: []string{"open"},
	Short:   "Create (if needed) and attach to a worktree session",
	Long:    `Create a worktree if it doesn't exist, then attach to a session. Argument can be a branch name, relative path, or absolute path. If additional arguments are provided, claude is launched with those arguments instead of a shell.`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return fmt.Errorf("must be run from inside a git repository: %w", err)
		}
		format := outputFormat
		if format == "" {
			format = "tap"
		}
		exec := executor.ShellExecutor{}
		var claudeArgs []string
		if len(args) >= 2 {
			claudeArgs = args[1:]
		}
		rp, err := worktree.ResolvePath(home, repoPath, args[0])
		if err != nil {
			return err
		}
		return shop.Attach(exec, rp, format, claudeArgs)
	},
}
```

Update `statusCmd`:
```go
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of repos and worktrees",
	Long:  `Scan from the current directory for git repos with .worktrees/ and display status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		format := outputFormat
		if format == "" {
			format = "table"
		}
		rows := status.CollectStatus(cwd)
		if len(rows) == 0 {
			log.Info("no repos found")
			return nil
		}
		if format == "tap" {
			status.RenderTap(rows, os.Stdout)
		} else {
			fmt.Println(status.Render(rows))
		}
		return nil
	},
}
```

Similarly update `cleanCmd` and `pullCmd` to use `cwd` instead of `home`.
Remove `createRepo` variable and the `--repo` flags from `init()`.
Remove `completions.Remote(home, os.Stdout)` from `completionsCmd`.

**Step 3: Verify compilation**

Run: `nix develop --command go build ./cmd/sweatshop/`
Expected: Build will fail because `status`, `completions`, `clean`, `pull`, and `merge` packages still use old signatures. That's expected — we'll fix them in the next tasks.

**Step 4: Commit (compile-broken is OK here, we'll fix in subsequent tasks)**

```
git add internal/shop/shop.go cmd/sweatshop/main.go
git commit -m "refactor: update shop and main for .worktrees/ layout, drop SSH"
```

---

### Task 4: Update `status` package

**Files:**
- Modify: `internal/status/status.go`

**Step 1: Rewrite `CollectRepoStatus` and `CollectStatus`**

```go
func CollectRepoStatus(repoPath string) []BranchStatus {
	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil
	}

	repoLabel := filepath.Base(repoPath)
	var rows []BranchStatus

	mainBranch, err := git.BranchCurrent(repoPath)
	if err == nil && mainBranch != "" {
		rows = append(rows, CollectBranchStatus(repoLabel, repoPath, mainBranch))
	}

	for _, wtPath := range worktree.ListWorktrees(repoPath) {
		branch := filepath.Base(wtPath)
		bs := CollectBranchStatus(repoLabel, wtPath, branch)
		bs.IsWorktree = true
		rows = append(rows, bs)
	}

	return rows
}

func CollectStatus(startDir string) []BranchStatus {
	var all []BranchStatus
	for _, repoPath := range worktree.ScanRepos(startDir) {
		rows := CollectRepoStatus(repoPath)
		all = append(all, rows...)
	}
	return all
}
```

Remove the `home` parameter. `CollectRepoStatus` now takes `repoPath` directly instead of `(home, engArea, repo)`.

**Step 2: Run tests**

Run: `nix develop --command go test ./internal/status/ -v`
Expected: PASS (after adjusting any test helpers)

**Step 3: Commit**

```
git add internal/status/status.go
git commit -m "refactor: status uses PWD-relative scanning with .worktrees/"
```

---

### Task 5: Update `completions` package

**Files:**
- Modify: `internal/completions/completions.go`
- Modify: `internal/completions/completions_test.go`

**Step 1: Rewrite tests for PWD-relative completions**

Replace all tests in `completions_test.go`:

```go
package completions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalListsRepos(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "myrepo/") {
		t.Errorf("expected repo listing, got %q", output)
	}
	if !strings.Contains(output, "new worktree") {
		t.Errorf("expected 'new worktree' description, got %q", output)
	}
}

func TestLocalListsExistingWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoPath, ".worktrees", "feature-x"), 0o755)
	os.WriteFile(
		filepath.Join(repoPath, ".worktrees", "feature-x", ".git"),
		[]byte("gitdir: ../../.git/worktrees/feature-x\n"),
		0o644,
	)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "myrepo/.worktrees/feature-x") {
		t.Errorf("expected existing worktree, got %q", output)
	}
	if !strings.Contains(output, "existing worktree") {
		t.Errorf("expected 'existing worktree' description, got %q", output)
	}
}

func TestLocalHandlesNoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestLocalOutputIsTabSeparated(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected output lines")
	}
	if !strings.Contains(lines[0], "\t") {
		t.Errorf("expected tab-separated output, got %q", lines[0])
	}
}

func TestLocalFromInsideRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoPath, ".worktrees", "feat"), 0o755)
	os.WriteFile(
		filepath.Join(repoPath, ".worktrees", "feat", ".git"),
		[]byte("gitdir: ../../.git/worktrees/feat\n"),
		0o644,
	)

	var buf bytes.Buffer
	// When called from inside a repo, should list that repo's worktrees
	Local(repoPath, &buf)

	output := buf.String()
	if !strings.Contains(output, "feat") {
		t.Errorf("expected worktree listing from inside repo, got %q", output)
	}
}
```

**Step 2: Rewrite `completions.go`**

```go
package completions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

// Local outputs tab-separated completion entries.
// startDir is the current working directory.
func Local(startDir string, w io.Writer) {
	gitPath := filepath.Join(startDir, ".git")
	if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
		// Inside a repo — list worktrees for this repo
		listRepoCompletions(startDir, "", w)
		return
	}

	// Parent of repos — scan children
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childPath := filepath.Join(startDir, entry.Name())
		gitPath := filepath.Join(childPath, ".git")
		if info, err := os.Stat(gitPath); err != nil || !info.IsDir() {
			continue
		}
		listRepoCompletions(childPath, entry.Name()+"/", w)
	}
}

func listRepoCompletions(repoPath, prefix string, w io.Writer) {
	repoName := filepath.Base(repoPath)
	if prefix == "" {
		prefix = repoName + "/"
	}

	// Always offer new worktree creation
	fmt.Fprintf(w, "%s\tnew worktree\n", prefix)

	// List existing worktrees
	for _, wtPath := range worktree.ListWorktrees(repoPath) {
		branch := filepath.Base(wtPath)
		fmt.Fprintf(w, "%s.worktrees/%s\texisting worktree\n", prefix, branch)
	}
}
```

Remove `Remote`, `remoteHosts`, `scanRemoteHost` functions.

**Step 3: Run tests**

Run: `nix develop --command go test ./internal/completions/ -v`
Expected: PASS

**Step 4: Commit**

```
git add internal/completions/completions.go internal/completions/completions_test.go
git commit -m "refactor: completions use PWD-relative scanning, drop remote"
```

---

### Task 6: Update `merge` package

**Files:**
- Modify: `internal/merge/merge.go`

**Step 1: Simplify to git-only detection**

Remove the convention path detection code path. The merge command already has a
git-based fallback that works:

```go
package merge

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/executor"
	"github.com/amarbel-llc/sweatshop/internal/git"
)

func Run(exec executor.Executor) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoPath, err := git.CommonDir(cwd)
	if err != nil {
		return fmt.Errorf("not in a worktree directory: %s", cwd)
	}

	branch, err := git.BranchCurrent(cwd)
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	log.Info("merging worktree", "worktree", branch)

	if err := git.RunPassthrough(repoPath, "merge", "--no-ff", branch, "-m", "Merge worktree: "+branch); err != nil {
		log.Error("merge failed, not removing worktree")
		return err
	}

	log.Info("removing worktree", "path", cwd)
	if err := git.RunPassthrough(repoPath, "worktree", "remove", cwd); err != nil {
		return err
	}

	log.Info("detaching from session")
	return exec.Detach()
}
```

Remove `worktree` import.

**Step 2: Verify compilation**

Run: `nix develop --command go build ./cmd/sweatshop/`

**Step 3: Commit**

```
git add internal/merge/merge.go
git commit -m "refactor: merge uses git-only detection, remove convention path"
```

---

### Task 7: Update `clean` package

**Files:**
- Modify: `internal/clean/clean.go`

**Step 1: Rewrite `scanWorktrees` for PWD-relative scanning**

Change `scanWorktrees` signature from `(home string)` to `(startDir string)`:

```go
func scanWorktrees(startDir string) []worktreeInfo {
	var worktrees []worktreeInfo

	for _, repoPath := range worktree.ScanRepos(startDir) {
		repoName := filepath.Base(repoPath)

		gitDir := filepath.Join(repoPath, ".git")
		if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
			continue
		}

		defaultBranch, err := git.DefaultBranch(repoPath)
		if err != nil || defaultBranch == "" {
			continue
		}

		for _, wtPath := range worktree.ListWorktrees(repoPath) {
			branch := filepath.Base(wtPath)
			ahead := git.CommitsAhead(wtPath, defaultBranch, branch)
			porcelain := git.StatusPorcelain(wtPath)

			worktrees = append(worktrees, worktreeInfo{
				repo:         repoName,
				branch:       branch,
				repoPath:     repoPath,
				worktreePath: wtPath,
				merged:       ahead == 0,
				dirty:        porcelain != "",
			})
		}
	}

	return worktrees
}
```

Remove `engArea` field from `worktreeInfo` struct. Update label formatting in
`Run` to use `wt.repo + "/.worktrees/" + styleCode.Render(wt.branch)`.

Update `Run` signature from `(home string, ...)` to `(startDir string, ...)`.

**Step 2: Verify compilation**

Run: `nix develop --command go build ./cmd/sweatshop/`

**Step 3: Commit**

```
git add internal/clean/clean.go
git commit -m "refactor: clean uses PWD-relative scanning with .worktrees/"
```

---

### Task 8: Update `pull` package

**Files:**
- Modify: `internal/pull/pull.go`

**Step 1: Rewrite scanning for PWD-relative**

Change `Run` signature from `(home string, dirty bool)` to `(startDir string, dirty bool)`.

Rewrite `scanRepos` and `scanWorktrees`:

```go
func scanRepos(startDir string) []repoInfo {
	var repos []repoInfo

	// Check if startDir is itself a repo
	gitPath := filepath.Join(startDir, ".git")
	if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
		porcelain := git.StatusPorcelain(startDir)
		return []repoInfo{{
			name:     filepath.Base(startDir),
			repoPath: startDir,
			dirty:    porcelain != "",
		}}
	}

	// Scan children
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(startDir, entry.Name())
		gitDir := filepath.Join(repoPath, ".git")
		if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
			continue
		}
		porcelain := git.StatusPorcelain(repoPath)
		repos = append(repos, repoInfo{
			name:     entry.Name(),
			repoPath: repoPath,
			dirty:    porcelain != "",
		})
	}
	return repos
}

func scanWorktrees(repos []repoInfo) []worktreeInfo {
	var worktrees []worktreeInfo
	for _, repo := range repos {
		for _, wtPath := range worktree.ListWorktrees(repo.repoPath) {
			branch := filepath.Base(wtPath)
			porcelain := git.StatusPorcelain(wtPath)
			worktrees = append(worktrees, worktreeInfo{
				repo:         repo.name,
				branch:       branch,
				repoPath:     repo.repoPath,
				worktreePath: wtPath,
				dirty:        porcelain != "",
			})
		}
	}
	return worktrees
}
```

Remove `engArea` field from `repoInfo` and `worktreeInfo`. Update labels in `Run`
to use `repo.name` and `wt.repo + "/.worktrees/" + wt.branch`.

Remove `home` parameter from `scanWorktrees`, pass repos directly.

Add `"github.com/amarbel-llc/sweatshop/internal/worktree"` import.

**Step 2: Verify compilation**

Run: `nix develop --command go build ./cmd/sweatshop/`

**Step 3: Commit**

```
git add internal/pull/pull.go
git commit -m "refactor: pull uses PWD-relative scanning with .worktrees/"
```

---

### Task 9: Full compilation and unit test pass

**Step 1: Build the project**

Run: `nix develop --command go build ./cmd/sweatshop/`
Expected: PASS — clean build, no errors.

**Step 2: Run all unit tests**

Run: `nix develop --command go test ./... -v`
Expected: PASS

**Step 3: Fix any remaining compilation issues**

Check for any leftover references to removed types/functions (`ParseTarget`,
`PathComponents`, `ParsePath`, `ShopKey`, `EngAreaDir`, `Convention`,
`OpenRemote`, `RepoPath` function, `WorktreePath` function). Fix as needed.

**Step 4: Commit any fixes**

```
git add -A
git commit -m "fix: resolve remaining compilation issues from refactor"
```

---

### Task 10: Update bats integration tests

**Files:**
- Modify: `tests/test_status.bats`
- Modify: `tests/test_completions.bats`
- Modify: `tests/test_sweatfile.bats`
- Modify: `tests/test_clean.bats`
- Modify: `tests/test_pull.bats`

**Step 1: Update `test_status.bats`**

Key changes:
- Worktrees now created at `$HOME/eng/repos/myrepo/.worktrees/feature-x` instead
  of `$HOME/eng/worktrees/myrepo/feature-x`
- Status must be run from the directory containing the repos (cd to `$HOME/eng/repos`)
- Update path assertions

```bash
function status_discovers_repos { # @test
  create_mock_repo "$HOME/eng/repos/repo-a"
  create_mock_repo "$HOME/eng/repos/repo-b"

  cd "$HOME/eng/repos"
  run sweatshop status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"repo-a"* ]]
  [[ "$output" == *"repo-b"* ]]
}

function status_handles_repos_with_worktrees { # @test
  create_mock_repo "$HOME/eng/repos/myrepo"

  local worktree_path="$HOME/eng/repos/myrepo/.worktrees/feature-x"
  mkdir -p "$(dirname "$worktree_path")"
  git -C "$HOME/eng/repos/myrepo" worktree add -q "$worktree_path"

  cd "$HOME/eng/repos"
  run sweatshop status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"myrepo"* ]]
  [[ "$output" == *"feature-x"* ]]
}

function status_from_inside_repo { # @test
  create_mock_repo "$HOME/eng/repos/myrepo"

  local worktree_path="$HOME/eng/repos/myrepo/.worktrees/feature-x"
  mkdir -p "$(dirname "$worktree_path")"
  git -C "$HOME/eng/repos/myrepo" worktree add -q "$worktree_path"

  cd "$HOME/eng/repos/myrepo"
  run sweatshop status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"feature-x"* ]]
}

function status_shows_no_repos_message { # @test
  cd "$HOME"
  run sweatshop status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"no repos found"* ]]
}
```

**Step 2: Update `test_completions.bats`**

Change paths from `eng/worktrees/myrepo/` to `.worktrees/` inside repos.
Run completions from inside repo or parent dir.

**Step 3: Update `test_sweatfile.bats`**

- Create global sweatfile at `$XDG_CONFIG_HOME/sweatshop/sweatfile` (or
  `$HOME/.config/sweatshop/sweatfile`)
- Worktrees created at `$HOME/eng/repos/testrepo/.worktrees/<branch>`
- `create` and `attach` called with just the branch name from inside the repo dir
- Update path assertions

**Step 4: Update `test_clean.bats`**

- Worktrees at `$HOME/eng/repos/myrepo/.worktrees/<branch>`
- Run clean from `$HOME/eng/repos`
- Update label assertions

**Step 5: Update `test_pull.bats`**

- Worktrees at `$HOME/eng/repos/myrepo/.worktrees/<branch>`
- Run pull from `$HOME/eng/repos`
- Update label assertions

**Step 6: Build and run bats tests**

Run: `just build && just test-bats`
Expected: PASS

**Step 7: Commit**

```
git add tests/
git commit -m "test: update bats tests for .worktrees/ layout"
```

---

### Task 11: Clean up dead code

**Files:**
- Modify: `internal/worktree/worktree.go` (if any old functions remain)
- Delete or modify: any files with orphaned remote/SSH code

**Step 1: Search for remaining references to removed functions**

Grep for: `ParseTarget`, `PathComponents`, `ParsePath`, `ShopKey`, `RepoPath(`,
`WorktreePath(`, `OpenRemote`, `EngAreaDir`, `Convention`, `ListRemote`,
`remoteHosts`, `scanRemoteHost`, `LoadMerged`, `LoadSingle`

**Step 2: Remove any dead code found**

**Step 3: Run full test suite**

Run: `just build && just test && just test-bats`
Expected: PASS

**Step 4: Commit**

```
git add -A
git commit -m "chore: remove dead code from convention path and SSH refactor"
```

---

### Task 12: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update the Architecture section**

Replace convention path references with `.worktrees/` descriptions:

- Remove `<eng_area>/worktrees/<repo>/<branch>` references
- Add `.worktrees/<branch>` inside repo
- Update command descriptions (create takes branch name or path, no --repo)
- Remove SSH/remote references
- Note PWD-relative scanning for status/clean/pull
- Update sweatfile loading description

**Step 2: Commit**

```
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for .worktrees/ layout"
```
