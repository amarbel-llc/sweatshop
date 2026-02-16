# Permission Tiers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add tiered Claude Code permission management to sweatshop — curated global/repo permission tiers, a PermissionRequest hook that auto-approves with visible banners, and post-session review for routing new approvals.

**Architecture:** New `internal/perms` package handles tier loading, permission matching, and JSON I/O. New `perms` cobra subcommand group (`check`, `review`, `list`, `edit`) wired in `cmd/sweatshop/main.go`. Post-session review integrated into existing `PostZmx` flow in `internal/attach/attach.go`.

**Tech Stack:** Go, cobra, huh (interactive prompts), encoding/json, filepath/glob-style matching

---

### Task 1: Permission Tier Loading and JSON I/O

**Files:**
- Create: `internal/perms/tiers.go`
- Create: `internal/perms/tiers_test.go`

**Step 1: Write the failing test**

```go
package perms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTierFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "global.json")
	os.WriteFile(path, []byte(`{"allow":["Read","Bash(go test:*)"]}`), 0o644)

	tier, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tier.Allow) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(tier.Allow))
	}
	if tier.Allow[0] != "Read" {
		t.Errorf("expected Read, got %q", tier.Allow[0])
	}
}

func TestLoadTierFileMissing(t *testing.T) {
	tier, err := LoadTierFile("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(tier.Allow) != 0 {
		t.Errorf("expected empty allow list, got %d", len(tier.Allow))
	}
}

func TestLoadTiers(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "global.json"), []byte(`{"allow":["Read"]}`), 0o644)
	os.MkdirAll(filepath.Join(dir, "repos"), 0o755)
	os.WriteFile(filepath.Join(dir, "repos", "myrepo.json"), []byte(`{"allow":["Bash(go test:*)"]}`), 0o644)

	rules := LoadTiers(dir, "myrepo")
	if len(rules) != 2 {
		t.Fatalf("expected 2 merged rules, got %d", len(rules))
	}
}

func TestSaveTierFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	tier := Tier{Allow: []string{"Read", "Edit"}}

	if err := SaveTierFile(path, tier); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, _ := LoadTierFile(path)
	if len(loaded.Allow) != 2 {
		t.Errorf("expected 2 rules after save+load, got %d", len(loaded.Allow))
	}
}

func TestAppendToTierFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, []byte(`{"allow":["Read"]}`), 0o644)

	if err := AppendToTierFile(path, "Edit"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, _ := LoadTierFile(path)
	if len(loaded.Allow) != 2 {
		t.Errorf("expected 2 rules, got %d", len(loaded.Allow))
	}
}

func TestAppendToTierFileNoDuplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, []byte(`{"allow":["Read"]}`), 0o644)

	AppendToTierFile(path, "Read")

	loaded, _ := LoadTierFile(path)
	if len(loaded.Allow) != 1 {
		t.Errorf("expected 1 rule (no duplicate), got %d", len(loaded.Allow))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/perms/ -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package perms

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Tier struct {
	Allow []string `json:"allow"`
}

func LoadTierFile(path string) (Tier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Tier{}, nil
		}
		return Tier{}, err
	}
	var t Tier
	if err := json.Unmarshal(data, &t); err != nil {
		return Tier{}, err
	}
	return t, nil
}

func SaveTierFile(path string, tier Tier) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tier, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func AppendToTierFile(path string, rule string) error {
	tier, err := LoadTierFile(path)
	if err != nil {
		return err
	}
	for _, r := range tier.Allow {
		if r == rule {
			return nil
		}
	}
	tier.Allow = append(tier.Allow, rule)
	return SaveTierFile(path, tier)
}

func TiersDir() string {
	if dir := os.Getenv("SWEATSHOP_PERMS_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sweatshop", "permissions")
}

func LoadTiers(tiersDir, repo string) []string {
	global, _ := LoadTierFile(filepath.Join(tiersDir, "global.json"))
	repoTier, _ := LoadTierFile(filepath.Join(tiersDir, "repos", repo+".json"))
	return append(global.Allow, repoTier.Allow...)
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/perms/ -v`
Expected: PASS — all 6 tests

**Step 5: Commit**

```
feat(perms): add tier file loading and JSON I/O
```

---

### Task 2: Permission Matching Engine

**Files:**
- Create: `internal/perms/match.go`
- Create: `internal/perms/match_test.go`

**Step 1: Write the failing test**

```go
package perms

import "testing"

func TestMatchExactTool(t *testing.T) {
	rules := []string{"Read", "Edit"}
	if !MatchesAnyRule(rules, "Read", nil) {
		t.Error("expected Read to match")
	}
	if MatchesAnyRule(rules, "Write", nil) {
		t.Error("expected Write not to match")
	}
}

func TestMatchBashWildcard(t *testing.T) {
	rules := []string{"Bash(go test:*)"}
	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "go test ./..."}) {
		t.Error("expected 'go test ./...' to match 'Bash(go test:*)'")
	}
	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "go build ./..."}) {
		t.Error("expected 'go build ./...' not to match 'Bash(go test:*)'")
	}
}

func TestMatchBashExact(t *testing.T) {
	rules := []string{"Bash(git status)"}
	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status"}) {
		t.Error("expected exact match")
	}
	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status -s"}) {
		t.Error("expected no match for longer command")
	}
}

func TestMatchMCPTool(t *testing.T) {
	rules := []string{"mcp__plugin_nix_nix__build"}
	if !MatchesAnyRule(rules, "mcp__plugin_nix_nix__build", nil) {
		t.Error("expected MCP tool to match")
	}
}

func TestMatchBashColonWildcard(t *testing.T) {
	rules := []string{"Bash(npm run:*)"}
	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "npm run build"}) {
		t.Error("expected 'npm run build' to match 'Bash(npm run:*)'")
	}
}

func TestMatchBashTrailingWildcard(t *testing.T) {
	rules := []string{"Bash(git *)"}
	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status"}) {
		t.Error("expected 'git status' to match 'Bash(git *)'")
	}
	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git log --oneline"}) {
		t.Error("expected 'git log --oneline' to match 'Bash(git *)'")
	}
}

func TestBuildPermissionString(t *testing.T) {
	got := BuildPermissionString("Bash", map[string]any{"command": "go test ./..."})
	if got != "Bash(go test ./...)" {
		t.Errorf("expected 'Bash(go test ./...)', got %q", got)
	}

	got = BuildPermissionString("Read", nil)
	if got != "Read" {
		t.Errorf("expected 'Read', got %q", got)
	}

	got = BuildPermissionString("Read", map[string]any{"file_path": "/tmp/foo"})
	if got != "Read(/tmp/foo)" {
		t.Errorf("expected 'Read(/tmp/foo)', got %q", got)
	}
}

func TestMatchingRuleName(t *testing.T) {
	rules := []string{"Read", "Bash(go test:*)"}
	name, ok := MatchingRule(rules, "Bash", map[string]any{"command": "go test ./..."})
	if !ok {
		t.Fatal("expected match")
	}
	if name != "Bash(go test:*)" {
		t.Errorf("expected 'Bash(go test:*)', got %q", name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/perms/ -v -run TestMatch`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
package perms

import (
	"path/filepath"
	"strings"
)

// BuildPermissionString constructs the Claude permission string from tool name and input.
func BuildPermissionString(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "Bash":
		if cmd, ok := toolInput["command"].(string); ok {
			return "Bash(" + cmd + ")"
		}
	case "Read", "Edit", "Write":
		if fp, ok := toolInput["file_path"].(string); ok {
			return toolName + "(" + fp + ")"
		}
	case "WebFetch":
		if url, ok := toolInput["url"].(string); ok {
			return toolName + "(" + url + ")"
		}
	}
	return toolName
}

// MatchesAnyRule checks if a tool invocation matches any rule in the list.
func MatchesAnyRule(rules []string, toolName string, toolInput map[string]any) bool {
	_, ok := MatchingRule(rules, toolName, toolInput)
	return ok
}

// MatchingRule returns the first matching rule and true, or ("", false) if none match.
func MatchingRule(rules []string, toolName string, toolInput map[string]any) (string, bool) {
	permStr := BuildPermissionString(toolName, toolInput)

	for _, rule := range rules {
		if matchRule(rule, toolName, permStr) {
			return rule, true
		}
	}
	return "", false
}

func matchRule(rule, toolName, permStr string) bool {
	// Exact match (e.g. "Read" == "Read", or "mcp__nix__build" == "mcp__nix__build")
	if rule == permStr || rule == toolName {
		return true
	}

	// Rule has parenthesized pattern: "Bash(go test:*)" or "Bash(git *)"
	if !strings.Contains(rule, "(") {
		return false
	}

	ruleToolName, rulePattern := parseRule(rule)
	if ruleToolName != toolName {
		return false
	}

	// Extract the actual value from the permission string
	_, actualValue := parseRule(permStr)
	if actualValue == "" {
		return false
	}

	return matchPattern(rulePattern, actualValue)
}

func parseRule(rule string) (toolName, pattern string) {
	idx := strings.IndexByte(rule, '(')
	if idx < 0 {
		return rule, ""
	}
	toolName = rule[:idx]
	pattern = rule[idx+1:]
	if len(pattern) > 0 && pattern[len(pattern)-1] == ')' {
		pattern = pattern[:len(pattern)-1]
	}
	return toolName, pattern
}

func matchPattern(pattern, value string) bool {
	// Claude uses two wildcard forms:
	// "Bash(go test:*)" — colon-star means "starts with 'go test'" (any suffix after space)
	// "Bash(git *)" — space-star means "starts with 'git '"
	// "Bash(git status)" — exact match

	// Handle ":*" suffix — match prefix up to the colon
	if strings.HasSuffix(pattern, ":*") {
		prefix := pattern[:len(pattern)-2]
		return value == prefix || strings.HasPrefix(value, prefix+" ")
	}

	// Use filepath.Match for glob-style patterns (handles * wildcards)
	if strings.ContainsAny(pattern, "*?[") {
		matched, _ := filepath.Match(pattern, value)
		if matched {
			return true
		}
		// filepath.Match doesn't match across path separators or spaces well
		// for "git *" matching "git log --oneline", use prefix matching
		if strings.Contains(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			prefix = strings.TrimSuffix(prefix, " ")
			return strings.HasPrefix(value, prefix+" ") || value == strings.TrimSpace(prefix)
		}
		return false
	}

	return pattern == value
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/perms/ -v -run TestMatch`
Expected: PASS — all match tests

**Step 5: Commit**

```
feat(perms): add permission matching engine
```

---

### Task 3: PermissionRequest Hook Handler (`perms check`)

**Files:**
- Create: `internal/perms/check.go`
- Create: `internal/perms/check_test.go`
- Modify: `cmd/sweatshop/main.go` (add perms subcommand group)

**Step 1: Write the failing test**

```go
package perms

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type HookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	CWD       string         `json:"cwd"`
}

func TestCheckMatchProducesAllow(t *testing.T) {
	tiersDir := t.TempDir()
	os.WriteFile(filepath.Join(tiersDir, "global.json"),
		[]byte(`{"allow":["Bash(go test:*)"]}`), 0o644)

	input := HookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "go test ./..."},
		CWD:       "/home/user/eng/worktrees/myrepo/feature",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	hso, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput")
	}
	decision, ok := hso["decision"].(map[string]any)
	if !ok {
		t.Fatal("expected decision")
	}
	if decision["behavior"] != "allow" {
		t.Errorf("expected allow, got %v", decision["behavior"])
	}

	if _, ok := result["systemMessage"]; !ok {
		t.Error("expected systemMessage for visibility")
	}
}

func TestCheckNoMatchProducesEmptyOutput(t *testing.T) {
	tiersDir := t.TempDir()
	os.WriteFile(filepath.Join(tiersDir, "global.json"),
		[]byte(`{"allow":["Read"]}`), 0o644)

	input := HookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "rm -rf /"},
		CWD:       "/home/user/eng/worktrees/myrepo/feature",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected empty output for non-match, got %q", out.String())
	}
}

func TestCheckUsesRepoTier(t *testing.T) {
	tiersDir := t.TempDir()
	os.WriteFile(filepath.Join(tiersDir, "global.json"),
		[]byte(`{"allow":["Read"]}`), 0o644)
	os.MkdirAll(filepath.Join(tiersDir, "repos"), 0o755)
	os.WriteFile(filepath.Join(tiersDir, "repos", "myrepo.json"),
		[]byte(`{"allow":["Bash(cargo test:*)"]}`), 0o644)

	input := HookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "cargo test"},
		CWD:       "/home/user/eng/worktrees/myrepo/feature",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Error("expected allow output from repo tier match")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/perms/ -v -run TestCheck`
Expected: FAIL — RunCheck not defined

**Step 3: Write minimal implementation**

```go
package perms

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type hookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	CWD       string         `json:"cwd"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecific `json:"hookSpecificOutput"`
	SystemMessage      string       `json:"systemMessage,omitempty"`
}

type hookSpecific struct {
	HookEventName string       `json:"hookEventName"`
	Decision      hookDecision `json:"decision"`
}

type hookDecision struct {
	Behavior string `json:"behavior"`
}

func RunCheck(r io.Reader, w io.Writer, tiersDir string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}

	repo := repoFromCWD(input.CWD)
	rules := LoadTiers(tiersDir, repo)

	rule, matched := MatchingRule(rules, input.ToolName, input.ToolInput)
	if !matched {
		return nil
	}

	permStr := BuildPermissionString(input.ToolName, input.ToolInput)
	tier := "global"
	if repo != "" {
		repoTier, _ := LoadTierFile(fmt.Sprintf("%s/repos/%s.json", tiersDir, repo))
		for _, r := range repoTier.Allow {
			if r == rule {
				tier = repo
				break
			}
		}
	}

	out := hookOutput{
		HookSpecificOutput: hookSpecific{
			HookEventName: "PermissionRequest",
			Decision:       hookDecision{Behavior: "allow"},
		},
		SystemMessage: fmt.Sprintf("[sweatshop] auto-approved: %s (%s tier)", permStr, tier),
	}

	return json.NewEncoder(w).Encode(out)
}

func repoFromCWD(cwd string) string {
	home, _ := os.UserHomeDir()
	rel := strings.TrimPrefix(cwd, home+"/")
	parts := strings.Split(rel, "/")
	// Match <eng_area>/worktrees/<repo>/... pattern
	if len(parts) >= 4 && parts[1] == "worktrees" {
		return parts[2]
	}
	// Match <eng_area>/repos/<repo>/... pattern
	if len(parts) >= 3 && parts[1] == "repos" {
		return parts[2]
	}
	return ""
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/perms/ -v -run TestCheck`
Expected: PASS — all 3 tests

**Step 5: Commit**

```
feat(perms): add PermissionRequest hook handler
```

---

### Task 4: Claude Settings File I/O

**Files:**
- Create: `internal/perms/settings.go`
- Create: `internal/perms/settings_test.go`

This handles reading/writing Claude's `.claude/settings.local.json` and diffing snapshots.

**Step 1: Write the failing test**

```go
package perms

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"permissions":{"allow":["Read","Edit","Bash(go test:*)"]}}`), 0o644)

	rules, err := LoadClaudeSettings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
}

func TestLoadClaudeSettingsMissing(t *testing.T) {
	rules, err := LoadClaudeSettings("/nonexistent/path")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(rules))
	}
}

func TestSaveClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")

	err := SaveClaudeSettings(path, []string{"Read", "Edit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rules, _ := LoadClaudeSettings(path)
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestDiffRules(t *testing.T) {
	before := []string{"Read", "Edit"}
	after := []string{"Read", "Edit", "Bash(go test:*)", "Write"}

	added := DiffRules(before, after)
	if len(added) != 2 {
		t.Fatalf("expected 2 new rules, got %d", len(added))
	}
	if added[0] != "Bash(go test:*)" || added[1] != "Write" {
		t.Errorf("unexpected diff: %v", added)
	}
}

func TestDiffRulesNoChanges(t *testing.T) {
	rules := []string{"Read", "Edit"}
	added := DiffRules(rules, rules)
	if len(added) != 0 {
		t.Errorf("expected 0 new rules, got %d", len(added))
	}
}

func TestRemoveRules(t *testing.T) {
	rules := []string{"Read", "Edit", "Write", "Glob"}
	toRemove := []string{"Edit", "Glob"}
	result := RemoveRules(rules, toRemove)
	if len(result) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(result))
	}
	if result[0] != "Read" || result[1] != "Write" {
		t.Errorf("unexpected result: %v", result)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/perms/ -v -run "TestLoad|TestSave|TestDiff|TestRemove"`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
package perms

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type claudeSettings struct {
	Permissions claudePermissions `json:"permissions"`
}

type claudePermissions struct {
	Allow []string `json:"allow"`
}

func LoadClaudeSettings(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var settings claudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings.Permissions.Allow, nil
}

func SaveClaudeSettings(path string, rules []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	settings := claudeSettings{
		Permissions: claudePermissions{Allow: rules},
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func DiffRules(before, after []string) []string {
	set := make(map[string]struct{}, len(before))
	for _, r := range before {
		set[r] = struct{}{}
	}
	var added []string
	for _, r := range after {
		if _, ok := set[r]; !ok {
			added = append(added, r)
		}
	}
	return added
}

func RemoveRules(rules, toRemove []string) []string {
	set := make(map[string]struct{}, len(toRemove))
	for _, r := range toRemove {
		set[r] = struct{}{}
	}
	var result []string
	for _, r := range rules {
		if _, ok := set[r]; !ok {
			result = append(result, r)
		}
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/perms/ -v -run "TestLoad|TestSave|TestDiff|TestRemove"`
Expected: PASS — all 6 tests

**Step 5: Commit**

```
feat(perms): add Claude settings I/O and diff utilities
```

---

### Task 5: Post-Session Review Logic

**Files:**
- Create: `internal/perms/review.go`
- Create: `internal/perms/review_test.go`

The review logic is separated from the huh UI so it can be tested. The huh prompts are a thin wrapper.

**Step 1: Write the failing test**

```go
package perms

import (
	"os"
	"path/filepath"
	"testing"
)

// ReviewDecision represents what to do with a new permission
type testReviewDecision struct {
	Rule   string
	Action string // "global", "repo", "keep", "discard"
}

func TestRouteDecisions(t *testing.T) {
	tiersDir := t.TempDir()
	os.WriteFile(filepath.Join(tiersDir, "global.json"), []byte(`{"allow":["Read"]}`), 0o644)
	os.MkdirAll(filepath.Join(tiersDir, "repos"), 0o755)
	os.WriteFile(filepath.Join(tiersDir, "repos", "myrepo.json"), []byte(`{"allow":[]}`), 0o644)

	worktreeDir := t.TempDir()
	settingsPath := filepath.Join(worktreeDir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":["Read","Edit","Bash(go test:*)"]}}`), 0o644)

	decisions := []ReviewDecision{
		{Rule: "Edit", Action: ReviewPromoteGlobal},
		{Rule: "Bash(go test:*)", Action: ReviewPromoteRepo},
	}

	err := RouteDecisions(tiersDir, "myrepo", settingsPath, decisions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check global tier got "Edit"
	global, _ := LoadTierFile(filepath.Join(tiersDir, "global.json"))
	found := false
	for _, r := range global.Allow {
		if r == "Edit" {
			found = true
		}
	}
	if !found {
		t.Error("expected Edit in global tier")
	}

	// Check repo tier got "Bash(go test:*)"
	repoTier, _ := LoadTierFile(filepath.Join(tiersDir, "repos", "myrepo.json"))
	found = false
	for _, r := range repoTier.Allow {
		if r == "Bash(go test:*)" {
			found = true
		}
	}
	if !found {
		t.Error("expected Bash(go test:*) in repo tier")
	}

	// Check settings.local.json had promoted rules removed
	remaining, _ := LoadClaudeSettings(settingsPath)
	for _, r := range remaining {
		if r == "Edit" || r == "Bash(go test:*)" {
			t.Errorf("promoted rule %q should have been removed from settings", r)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/perms/ -v -run TestRoute`
Expected: FAIL — ReviewDecision, RouteDecisions not defined

**Step 3: Write minimal implementation**

```go
package perms

import "path/filepath"

const (
	ReviewPromoteGlobal = "global"
	ReviewPromoteRepo   = "repo"
	ReviewKeep          = "keep"
	ReviewDiscard       = "discard"
)

type ReviewDecision struct {
	Rule   string
	Action string
}

func RouteDecisions(tiersDir, repo, settingsPath string, decisions []ReviewDecision) error {
	var toRemoveFromSettings []string

	for _, d := range decisions {
		switch d.Action {
		case ReviewPromoteGlobal:
			if err := AppendToTierFile(filepath.Join(tiersDir, "global.json"), d.Rule); err != nil {
				return err
			}
			toRemoveFromSettings = append(toRemoveFromSettings, d.Rule)
		case ReviewPromoteRepo:
			if err := AppendToTierFile(filepath.Join(tiersDir, "repos", repo+".json"), d.Rule); err != nil {
				return err
			}
			toRemoveFromSettings = append(toRemoveFromSettings, d.Rule)
		case ReviewDiscard:
			toRemoveFromSettings = append(toRemoveFromSettings, d.Rule)
		case ReviewKeep:
			// leave in settings.local.json
		}
	}

	if len(toRemoveFromSettings) == 0 {
		return nil
	}

	current, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}

	remaining := RemoveRules(current, toRemoveFromSettings)
	return SaveClaudeSettings(settingsPath, remaining)
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/perms/ -v -run TestRoute`
Expected: PASS

**Step 5: Commit**

```
feat(perms): add post-session review routing logic
```

---

### Task 6: Wire Cobra Subcommands

**Files:**
- Create: `internal/perms/cmd.go`
- Modify: `cmd/sweatshop/main.go` (add perms command group)

**Step 1: Write `internal/perms/cmd.go`**

This file contains the cobra command constructors. The `check` command reads stdin and writes stdout (hook protocol). The `review` command takes a sweatshop path and runs the interactive review. The `list` command prints tier contents. The `edit` command opens `$EDITOR`.

```go
package perms

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func NewPermsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perms",
		Short: "Manage Claude Code permission tiers",
	}
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newEditCmd())
	return cmd
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "check",
		Short:  "PermissionRequest hook handler (reads stdin, writes stdout)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunCheck(os.Stdin, os.Stdout, TiersDir())
		},
	}
}

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <sweatshop-path>",
		Short: "Review new permissions from a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunReviewInteractive(args[0])
		},
	}
}

func newListCmd() *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show curated permission tiers",
		RunE: func(cmd *cobra.Command, args []string) error {
			tiersDir := TiersDir()
			global, _ := LoadTierFile(filepath.Join(tiersDir, "global.json"))
			fmt.Println("Global tier:")
			for _, r := range global.Allow {
				fmt.Printf("  %s\n", r)
			}

			if repo != "" {
				repoTier, _ := LoadTierFile(filepath.Join(tiersDir, "repos", repo+".json"))
				fmt.Printf("\nRepo tier (%s):\n", repo)
				for _, r := range repoTier.Allow {
					fmt.Printf("  %s\n", r)
				}
			} else {
				// List all repo tiers
				reposDir := filepath.Join(tiersDir, "repos")
				entries, _ := os.ReadDir(reposDir)
				for _, e := range entries {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					name := strings.TrimSuffix(e.Name(), ".json")
					repoTier, _ := LoadTierFile(filepath.Join(reposDir, e.Name()))
					fmt.Printf("\nRepo tier (%s):\n", name)
					for _, r := range repoTier.Allow {
						fmt.Printf("  %s\n", r)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "show tier for specific repo")
	return cmd
}

func newEditCmd() *cobra.Command {
	var global bool
	var repo string
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open a permission tier in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			tiersDir := TiersDir()
			var path string
			if global {
				path = filepath.Join(tiersDir, "global.json")
			} else if repo != "" {
				path = filepath.Join(tiersDir, "repos", repo+".json")
			} else {
				return fmt.Errorf("specify --global or --repo <name>")
			}

			// Create file with empty tier if it doesn't exist
			if _, err := os.Stat(path); os.IsNotExist(err) {
				SaveTierFile(path, Tier{Allow: []string{}})
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "edit global tier")
	cmd.Flags().StringVar(&repo, "repo", "", "edit repo-specific tier")
	return cmd
}

// RunReviewInteractive runs the interactive post-session review.
// Called from the PostZmx flow or directly via `sweatshop perms review`.
func RunReviewInteractive(sweatshopPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	parts := strings.Split(sweatshopPath, "/")
	if len(parts) < 4 || parts[1] != "worktrees" {
		return nil
	}
	repo := parts[2]

	worktreePath := filepath.Join(home, sweatshopPath)
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")
	snapshotPath := filepath.Join(worktreePath, ".claude", ".settings-snapshot.json")

	snapshotRules, _ := LoadClaudeSettings(snapshotPath)
	currentRules, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		return nil
	}

	newRules := DiffRules(snapshotRules, currentRules)
	if len(newRules) == 0 {
		return nil
	}

	var decisions []ReviewDecision
	for _, rule := range newRules {
		var action string
		err := huh.NewSelect[string]().
			Title(fmt.Sprintf("New permission: %s", rule)).
			Options(
				huh.NewOption("Promote to global (all repos)", ReviewPromoteGlobal),
				huh.NewOption(fmt.Sprintf("Promote to %s (this repo)", repo), ReviewPromoteRepo),
				huh.NewOption("Keep for this worktree only", ReviewKeep),
				huh.NewOption("Discard", ReviewDiscard),
			).
			Value(&action).
			Run()
		if err != nil {
			return nil
		}
		decisions = append(decisions, ReviewDecision{Rule: rule, Action: action})
	}

	return RouteDecisions(TiersDir(), repo, settingsPath, decisions)
}

// SnapshotSettings saves a copy of the current settings for later diffing.
func SnapshotSettings(worktreePath string) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")
	snapshotPath := filepath.Join(worktreePath, ".claude", ".settings-snapshot.json")

	rules, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		return nil
	}
	return SaveClaudeSettings(snapshotPath, rules)
}

// CleanupSnapshot removes the snapshot file after review.
func CleanupSnapshot(worktreePath string) {
	os.Remove(filepath.Join(worktreePath, ".claude", ".settings-snapshot.json"))
}
```

**Step 2: Wire into `cmd/sweatshop/main.go`**

Add import and command registration:

In imports, add:
```go
"github.com/amarbel-llc/sweatshop/internal/perms"
```

In `init()`, add:
```go
rootCmd.AddCommand(perms.NewPermsCmd())
```

**Step 3: Run all tests**

Run: `nix develop --command go test ./... -v`
Expected: PASS — all tests including new perms package

**Step 4: Build to verify compilation**

Run: `nix develop --command go build -o build/sweatshop ./cmd/sweatshop`
Expected: Compiles without error

**Step 5: Commit**

```
feat(perms): wire cobra subcommands (check, review, list, edit)
```

---

### Task 7: Integrate into PostZmx Flow

**Files:**
- Modify: `internal/attach/attach.go` (add snapshot before zmx, review after zmx)

**Step 1: Add snapshot before zmx attach in `Existing()`**

In `internal/attach/attach.go`, in the `Existing` function, before the zmx command runs, add the snapshot call. After the zmx command and before `PostZmx`, the review happens inside PostZmx.

Changes to `Existing()` (lines 31-45):

```go
func Existing(sweatshopPath, format string, claudeArgs []string) error {
	home, _ := os.UserHomeDir()
	worktreePath := worktree.WorktreePath(home, sweatshopPath)
	perms.SnapshotSettings(worktreePath)

	zmxArgs := []string{"attach", sweatshopPath}
	// ... rest unchanged ...

	return PostZmx(sweatshopPath, format)
}
```

Same for `ToPath()` — add snapshot after worktree creation, before zmx.

Changes to `PostZmx()` — add review step between zmx return and the rebase/merge prompts. Insert after the `commitsAhead`/`worktreeStatus` check but before `chooseAction`:

```go
// Review new permissions before post-zmx git workflow
if err := perms.RunReviewInteractive(sweatshopPath); err != nil {
	log.Warn("permission review failed", "error", err)
}
perms.CleanupSnapshot(worktreePath)
```

**Step 2: Add import**

Add to imports in attach.go:
```go
"github.com/amarbel-llc/sweatshop/internal/perms"
```

**Step 3: Build and test**

Run: `nix develop --command go build -o build/sweatshop ./cmd/sweatshop && nix develop --command go test ./... -v`
Expected: Compiles and all tests pass

**Step 4: Commit**

```
feat(perms): integrate snapshot and review into attach flow
```

---

### Task 8: Bats Integration Tests

**Files:**
- Create: `tests/test_perms.bats`

**Step 1: Write the integration tests**

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path
}

function perms_check_approves_matching_global_rule { # @test
  mkdir -p "$HOME/.config/sweatshop/permissions"
  echo '{"allow":["Bash(go test:*)"]}' > "$HOME/.config/sweatshop/permissions/global.json"

  result=$(echo '{"tool_name":"Bash","tool_input":{"command":"go test ./..."},"cwd":"'"$HOME"'/eng/worktrees/myrepo/feat"}' \
    | SWEATSHOP_PERMS_DIR="$HOME/.config/sweatshop/permissions" sweatshop perms check)

  [[ "$result" == *'"behavior":"allow"'* ]]
  [[ "$result" == *'"systemMessage"'* ]]
  [[ "$result" == *'global tier'* ]]
}

function perms_check_passes_through_non_matching { # @test
  mkdir -p "$HOME/.config/sweatshop/permissions"
  echo '{"allow":["Read"]}' > "$HOME/.config/sweatshop/permissions/global.json"

  result=$(echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"cwd":"'"$HOME"'/eng/worktrees/myrepo/feat"}' \
    | SWEATSHOP_PERMS_DIR="$HOME/.config/sweatshop/permissions" sweatshop perms check)

  [[ -z "$result" ]]
}

function perms_check_uses_repo_tier { # @test
  mkdir -p "$HOME/.config/sweatshop/permissions/repos"
  echo '{"allow":[]}' > "$HOME/.config/sweatshop/permissions/global.json"
  echo '{"allow":["Bash(cargo test:*)"]}' > "$HOME/.config/sweatshop/permissions/repos/myrepo.json"

  result=$(echo '{"tool_name":"Bash","tool_input":{"command":"cargo test"},"cwd":"'"$HOME"'/eng/worktrees/myrepo/feat"}' \
    | SWEATSHOP_PERMS_DIR="$HOME/.config/sweatshop/permissions" sweatshop perms check)

  [[ "$result" == *'"behavior":"allow"'* ]]
  [[ "$result" == *'myrepo tier'* ]]
}

function perms_list_shows_tiers { # @test
  mkdir -p "$HOME/.config/sweatshop/permissions/repos"
  echo '{"allow":["Read","Glob"]}' > "$HOME/.config/sweatshop/permissions/global.json"
  echo '{"allow":["Bash(go test:*)"]}' > "$HOME/.config/sweatshop/permissions/repos/myrepo.json"

  run env SWEATSHOP_PERMS_DIR="$HOME/.config/sweatshop/permissions" sweatshop perms list
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"Global tier"* ]]
  [[ "$output" == *"Read"* ]]
  [[ "$output" == *"myrepo"* ]]
  [[ "$output" == *"Bash(go test:*)"* ]]
}

function perms_check_handles_empty_tiers_dir { # @test
  result=$(echo '{"tool_name":"Read","tool_input":{},"cwd":"'"$HOME"'/eng/worktrees/myrepo/feat"}' \
    | SWEATSHOP_PERMS_DIR="$HOME/.config/sweatshop/permissions" sweatshop perms check)

  [[ -z "$result" ]]
}
```

**Step 2: Build and run bats tests**

Run: `nix develop --command go build -o build/sweatshop ./cmd/sweatshop && nix develop --command bats tests/test_perms.bats`
Expected: PASS — all 5 integration tests

**Step 3: Commit**

```
test(perms): add bats integration tests for permission tiers
```

---

### Task 9: Update gomod2nix and Nix Build

**Step 1: Update go.mod if needed**

Run: `nix develop --command go mod tidy`

**Step 2: Regenerate gomod2nix.toml**

Run: `nix develop --command gomod2nix`

**Step 3: Build with nix**

Run: `nix build`
Expected: Build succeeds

**Step 4: Run full test suite**

Run: `nix develop --command go test ./... -v && nix develop --command bats tests/`
Expected: All tests pass

**Step 5: Commit**

```
chore: update gomod2nix.toml for perms package
```

---

### Task 10: Seed Initial Global Tier

Create a starter global tier with the universally safe permissions from the current rcm-worktrees overlay, so the hook has something to work with immediately.

**Step 1: Create the initial global tier file**

The file goes at `~/.config/sweatshop/permissions/global.json`. Extract the safe, non-path-specific rules from the current `rcm-worktrees/claude/settings.local.json`:

```json
{
  "allow": [
    "Read",
    "Edit",
    "Glob",
    "Bash(git add:*)",
    "Bash(git commit:*)",
    "Bash(git log:*)",
    "Bash(git stash:*)",
    "Bash(git rebase:*)",
    "Bash(ls:*)",
    "Bash(grep:*)",
    "Bash(just:*)",
    "Bash(just test:*)",
    "Bash(just build:*)",
    "mcp__plugin_grit_grit__add",
    "mcp__plugin_grit_grit__diff",
    "mcp__plugin_grit_grit__commit",
    "mcp__plugin_grit_grit__status",
    "mcp__plugin_grit_grit__log",
    "mcp__plugin_grit_grit__show",
    "mcp__plugin_grit_grit__blame",
    "mcp__plugin_grit_grit__branch_list",
    "mcp__plugin_grit_grit__fetch",
    "mcp__plugin_grit_grit__git_rev_parse",
    "mcp__plugin_lux_lux__document_symbols",
    "mcp__plugin_lux_lux__hover",
    "mcp__plugin_lux_lux__completion",
    "mcp__plugin_lux_lux__definition",
    "mcp__plugin_lux_lux__references",
    "mcp__plugin_lux_lux__workspace_symbols",
    "mcp__plugin_lux_lux__diagnostics",
    "mcp__plugin_nix_nix__build",
    "mcp__plugin_nix_nix__develop_run",
    "mcp__plugin_nix_nix__flake_show",
    "mcp__plugin_nix_nix__flake_check",
    "mcp__plugin_nix_nix__flake_metadata",
    "mcp__plugin_nix_nix__flake_update",
    "mcp__plugin_nix_nix__store_cat",
    "mcp__plugin_nix_nix__store_ls",
    "mcp__plugin_nix_nix__derivation_show"
  ]
}
```

This is a manual step — review the list and adjust to taste before saving.

**Step 2: Verify hook works end-to-end**

Run: `echo '{"tool_name":"Read","tool_input":{},"cwd":"'$HOME'/eng/worktrees/sweatshop/perms"}' | sweatshop perms check`
Expected: JSON output with `"behavior":"allow"` and systemMessage

**Step 3: Register the hook in `~/.claude/settings.json`**

Add under `"hooks"`:
```json
"PermissionRequest": [{
  "matcher": ".*",
  "hooks": [{
    "type": "command",
    "command": "sweatshop perms check",
    "timeout": 5
  }]
}]
```

This is a manual step — the user registers the hook after testing.
