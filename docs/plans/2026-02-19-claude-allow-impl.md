# Claude Allow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the inlined JSON blob in the sweatfile with a first-class `claude_allow` field, scope Edit/Write to the worktree directory, and fix path syntax to match Claude Code's parser.

**Architecture:** Add `ClaudeAllow []string` to the `Sweatfile` struct with the same nil/empty/non-empty merge semantics as other array fields. Move settings.local.json generation from a `[files]` entry + `injectWorktreePerms()` into a new `ApplyClaudeSettings()` function in `apply.go`. The `Apply()` signature gains a `worktreePath` parameter it already has, plus the caller passes the absolute worktree path for scoped rule generation.

**Tech Stack:** Go 1.23, BurntSushi/toml, encoding/json, go test

---

### Task 1: Add `ClaudeAllow` field to Sweatfile struct and merge logic

**Files:**
- Modify: `internal/sweatfile/sweatfile.go:17-22` (Sweatfile struct)
- Modify: `internal/sweatfile/sweatfile.go:43-82` (Merge function)
- Test: `internal/sweatfile/sweatfile_test.go`

**Step 1: Write the failing tests**

Add three tests to `internal/sweatfile/sweatfile_test.go`:

```go
func TestParseClaudeAllow(t *testing.T) {
	input := `
claude_allow = ["Read", "Bash(git *)"]
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.ClaudeAllow) != 2 {
		t.Fatalf("expected 2 claude_allow rules, got %v", sf.ClaudeAllow)
	}
	if sf.ClaudeAllow[0] != "Read" || sf.ClaudeAllow[1] != "Bash(git *)" {
		t.Errorf("claude_allow: got %v", sf.ClaudeAllow)
	}
}

func TestMergeClaudeAllowAppends(t *testing.T) {
	base := Sweatfile{ClaudeAllow: []string{"Read", "Glob"}}
	repo := Sweatfile{ClaudeAllow: []string{"Bash(go test:*)"}}
	merged := Merge(base, repo)
	if len(merged.ClaudeAllow) != 3 {
		t.Fatalf("expected 3 claude_allow rules, got %v", merged.ClaudeAllow)
	}
	if merged.ClaudeAllow[2] != "Bash(go test:*)" {
		t.Errorf("expected appended rule, got %v", merged.ClaudeAllow)
	}
}

func TestMergeClaudeAllowClear(t *testing.T) {
	base := Sweatfile{ClaudeAllow: []string{"Read", "Glob"}}
	repo := Sweatfile{ClaudeAllow: []string{}}
	merged := Merge(base, repo)
	if len(merged.ClaudeAllow) != 0 {
		t.Errorf("expected cleared claude_allow, got %v", merged.ClaudeAllow)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test ./internal/sweatfile/ -run TestParseClaudeAllow -v`
Expected: FAIL — `ClaudeAllow` field does not exist

**Step 3: Add the field and merge logic**

In `internal/sweatfile/sweatfile.go`, add `ClaudeAllow` to the struct:

```go
type Sweatfile struct {
	GitExcludes []string             `toml:"git_excludes"`
	ClaudeAllow []string             `toml:"claude_allow"`
	Env         map[string]string    `toml:"env"`
	Files       map[string]FileEntry `toml:"files"`
	Setup       []string             `toml:"setup"`
}
```

In the `Merge` function, add the same nil/empty/non-empty block used by
`GitExcludes` and `Setup`:

```go
if repo.ClaudeAllow != nil {
	if len(repo.ClaudeAllow) == 0 {
		merged.ClaudeAllow = []string{}
	} else {
		merged.ClaudeAllow = append(base.ClaudeAllow, repo.ClaudeAllow...)
	}
}
```

Place this block after the `GitExcludes` block and before the `Setup` block.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./internal/sweatfile/ -run "TestParseClaudeAllow|TestMergeClaudeAllow" -v`
Expected: PASS

**Step 5: Run all sweatfile tests**

Run: `nix develop --command go test ./internal/sweatfile/ -v`
Expected: All pass

**Step 6: Commit**

```
git add internal/sweatfile/sweatfile.go internal/sweatfile/sweatfile_test.go
git commit -m "feat: add ClaudeAllow field to Sweatfile struct"
```

---

### Task 2: Add `ApplyClaudeSettings` function

**Files:**
- Modify: `internal/sweatfile/apply.go` (add `ApplyClaudeSettings` function)
- Test: `internal/sweatfile/apply_test.go`

**Step 1: Write the failing tests**

Add to `internal/sweatfile/apply_test.go`:

```go
func TestApplyClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	rules := []string{"Read", "Glob", "Bash(git *)"}

	err := ApplyClaudeSettings(dir, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	if permsMap == nil {
		t.Fatal("expected permissions key")
	}

	allowRaw, _ := permsMap["allow"].([]any)
	if len(allowRaw) != 5 {
		t.Fatalf("expected 5 rules (3 sweatfile + 2 scoped), got %d: %v", len(allowRaw), allowRaw)
	}

	// First 3 are from sweatfile
	for i, want := range rules {
		got, _ := allowRaw[i].(string)
		if got != want {
			t.Errorf("rule %d: got %q, want %q", i, got, want)
		}
	}

	// Last 2 are auto-injected scoped rules
	editRule, _ := allowRaw[3].(string)
	writeRule, _ := allowRaw[4].(string)

	wantEdit := "Edit(//" + dir + "/**)"
	wantWrite := "Write(//" + dir + "/**)"
	if editRule != wantEdit {
		t.Errorf("edit rule: got %q, want %q", editRule, wantEdit)
	}
	if writeRule != wantWrite {
		t.Errorf("write rule: got %q, want %q", writeRule, wantWrite)
	}
}

func TestApplyClaudeSettingsEmpty(t *testing.T) {
	dir := t.TempDir()

	err := ApplyClaudeSettings(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var doc map[string]any
	json.Unmarshal(data, &doc)
	permsMap, _ := doc["permissions"].(map[string]any)
	allowRaw, _ := permsMap["allow"].([]any)

	// Even with no sweatfile rules, the 2 scoped rules are injected
	if len(allowRaw) != 2 {
		t.Fatalf("expected 2 scoped rules, got %d: %v", len(allowRaw), allowRaw)
	}
}

func TestApplyClaudeSettingsPreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	existing := map[string]any{
		"mcpServers": map[string]any{"test": true},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), data, 0o644)

	err := ApplyClaudeSettings(dir, []string{"Read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(result, &doc)

	if _, ok := doc["mcpServers"]; !ok {
		t.Error("expected mcpServers key to be preserved")
	}
}
```

Add `"encoding/json"` to the import block in `apply_test.go`.

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test ./internal/sweatfile/ -run TestApplyClaudeSettings -v`
Expected: FAIL — `ApplyClaudeSettings` not defined

**Step 3: Implement `ApplyClaudeSettings`**

Add to `internal/sweatfile/apply.go`:

```go
func ApplyClaudeSettings(worktreePath string, rules []string) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")

	var doc map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &doc)
	}
	if doc == nil {
		doc = make(map[string]any)
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	if permsMap == nil {
		permsMap = make(map[string]any)
	}

	allRules := append([]string{}, rules...)
	allRules = append(allRules,
		"Edit(//"+worktreePath+"/**)",
		"Write(//"+worktreePath+"/**)",
	)

	permsMap["allow"] = allRules
	doc["permissions"] = permsMap

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
```

Add `"encoding/json"` to the import block in `apply.go`.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./internal/sweatfile/ -run TestApplyClaudeSettings -v`
Expected: PASS

**Step 5: Run all sweatfile tests**

Run: `nix develop --command go test ./internal/sweatfile/ -v`
Expected: All pass

**Step 6: Commit**

```
git add internal/sweatfile/apply.go internal/sweatfile/apply_test.go
git commit -m "feat: add ApplyClaudeSettings for scoped permission injection"
```

---

### Task 3: Wire `ApplyClaudeSettings` into `Apply` and remove `injectWorktreePerms`

**Files:**
- Modify: `internal/sweatfile/apply.go:20-45` (Apply function)
- Modify: `internal/worktree/worktree.go:61-82` (Create function)
- Modify: `internal/worktree/worktree.go:84-153` (remove injectWorktreePerms, appendUnique)
- Modify: `internal/worktree/worktree_test.go:82-183` (remove inject tests)

**Step 1: Add `ApplyClaudeSettings` call to `Apply()`**

In `internal/sweatfile/apply.go`, add the call after `RunSetup` and before the
final return:

```go
func Apply(worktreePath string, sf Sweatfile) error {
	// ... existing excludes, files, env, setup steps ...

	if err := ApplyClaudeSettings(worktreePath, sf.ClaudeAllow); err != nil {
		return fmt.Errorf("applying claude settings: %w", err)
	}

	return nil
}
```

**Step 2: Remove `injectWorktreePerms` call from `worktree.Create()`**

In `internal/worktree/worktree.go`, change the end of `Create()` from:

```go
	if err := sweatfile.Apply(worktreePath, sf); err != nil {
		return err
	}

	return injectWorktreePerms(worktreePath)
```

To:

```go
	return sweatfile.Apply(worktreePath, sf)
```

**Step 3: Delete `injectWorktreePerms` and `appendUnique` functions**

Remove the `injectWorktreePerms` function (lines 84-131) and `appendUnique`
function (lines 141-153) from `internal/worktree/worktree.go`.

Remove the `"encoding/json"`, `"errors"`, and `"io/fs"` imports from
`worktree.go` (they were only used by `injectWorktreePerms`). Keep `"fmt"`,
`"os"`, `"path/filepath"`, `"strings"`, and the internal imports.

**Step 4: Update worktree tests**

In `internal/worktree/worktree_test.go`, remove the three tests that tested
`injectWorktreePerms`:

- `TestInjectWorktreePerms` (lines 82-121)
- `TestInjectWorktreePermsPreservesExisting` (lines 123-160)
- `TestInjectWorktreePermsIdempotent` (lines 162-183)

Remove the `"encoding/json"` import from `worktree_test.go` (only used by
inject tests). Keep `"os"`, `"path/filepath"`, `"testing"`.

**Step 5: Run all tests**

Run: `nix develop --command go test ./internal/... -v`
Expected: All pass

**Step 6: Run full build**

Run: `nix develop --command go build ./...`
Expected: Build succeeds with no errors

**Step 7: Commit**

```
git add internal/sweatfile/apply.go internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "refactor: wire ApplyClaudeSettings into Apply, remove injectWorktreePerms"
```

---

### Task 4: Final verification

**Step 1: Run all Go tests**

Run: `nix develop --command go test ./... -v`
Expected: All pass

**Step 2: Run gofumpt**

Run: `nix develop --command gofumpt -l .`
Expected: No output (all files formatted)

If any files are listed, run: `nix develop --command gofumpt -w .` and include
them in the commit.

**Step 3: Run nix build**

Run: `nix build --show-trace`
Expected: Build succeeds

**Step 4: Run nix flake check**

Run: `nix flake check`
Expected: Check passes

**Step 5: Commit any formatting fixes**

If gofumpt found issues:
```
git add -A
git commit -m "style: fix gofumpt formatting"
```
