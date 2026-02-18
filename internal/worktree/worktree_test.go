package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTargetLocal(t *testing.T) {
	target := ParseTarget("eng/worktrees/myrepo/mybranch")
	if target.Host != "" {
		t.Errorf("expected empty host, got %q", target.Host)
	}
	if target.Path != "eng/worktrees/myrepo/mybranch" {
		t.Errorf("expected path eng/worktrees/myrepo/mybranch, got %q", target.Path)
	}
}

func TestParseTargetRemote(t *testing.T) {
	target := ParseTarget("vm-host:eng/worktrees/myrepo/mybranch")
	if target.Host != "vm-host" {
		t.Errorf("expected host vm-host, got %q", target.Host)
	}
	if target.Path != "eng/worktrees/myrepo/mybranch" {
		t.Errorf("expected path eng/worktrees/myrepo/mybranch, got %q", target.Path)
	}
}

func TestParseTargetNoColon(t *testing.T) {
	target := ParseTarget("simple/path")
	if target.Host != "" {
		t.Errorf("expected empty host, got %q", target.Host)
	}
	if target.Path != "simple/path" {
		t.Errorf("expected path simple/path, got %q", target.Path)
	}
}

func TestParseTargetPreservesRemotePath(t *testing.T) {
	target := ParseTarget("myhost:eng2/worktrees/dodder/feature-x")
	if target.Host != "myhost" {
		t.Errorf("expected host myhost, got %q", target.Host)
	}
	if target.Path != "eng2/worktrees/dodder/feature-x" {
		t.Errorf("expected path eng2/worktrees/dodder/feature-x, got %q", target.Path)
	}
}

func TestParsePathValid(t *testing.T) {
	comp, err := ParsePath("eng/worktrees/myrepo/feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.EngArea != "eng" {
		t.Errorf("expected eng, got %q", comp.EngArea)
	}
	if comp.Repo != "myrepo" {
		t.Errorf("expected myrepo, got %q", comp.Repo)
	}
	if comp.Worktree != "feature-x" {
		t.Errorf("expected feature-x, got %q", comp.Worktree)
	}
}

func TestShopKey(t *testing.T) {
	comp := PathComponents{EngArea: "eng", Repo: "purse-first", Worktree: "other-marketplaces"}
	got := comp.ShopKey()
	want := "eng/purse-first/other-marketplaces"
	if got != want {
		t.Errorf("ShopKey() = %q, want %q", got, want)
	}
}

func TestParsePathInvalid(t *testing.T) {
	_, err := ParsePath("eng/repos/myrepo")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestInjectWorktreePerms(t *testing.T) {
	dir := t.TempDir()

	if err := injectWorktreePerms(dir); err != nil {
		t.Fatalf("injectWorktreePerms: %v", err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
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
	if len(allowRaw) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(allowRaw))
	}

	expected := []string{
		"Read(" + dir + "/*)",
		"Write(" + dir + "/*)",
		"Edit(" + dir + "/*)",
	}
	for i, want := range expected {
		got, _ := allowRaw[i].(string)
		if got != want {
			t.Errorf("rule %d: got %q, want %q", i, got, want)
		}
	}
}

func TestInjectWorktreePermsPreservesExisting(t *testing.T) {
	dir := t.TempDir()

	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	existing := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash(git status)"},
		},
		"mcpServers": map[string]any{},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), data, 0o644)

	if err := injectWorktreePerms(dir); err != nil {
		t.Fatalf("injectWorktreePerms: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(result, &doc)

	if _, ok := doc["mcpServers"]; !ok {
		t.Error("expected mcpServers key to be preserved")
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	allowRaw, _ := permsMap["allow"].([]any)
	if len(allowRaw) != 4 {
		t.Fatalf("expected 4 rules (1 existing + 3 new), got %d", len(allowRaw))
	}

	first, _ := allowRaw[0].(string)
	if first != "Bash(git status)" {
		t.Errorf("expected existing rule first, got %q", first)
	}
}

func TestInjectWorktreePermsIdempotent(t *testing.T) {
	dir := t.TempDir()

	if err := injectWorktreePerms(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := injectWorktreePerms(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}

	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	data, _ := os.ReadFile(settingsPath)
	var doc map[string]any
	json.Unmarshal(data, &doc)

	permsMap, _ := doc["permissions"].(map[string]any)
	allowRaw, _ := permsMap["allow"].([]any)
	if len(allowRaw) != 3 {
		t.Fatalf("expected 3 rules after double inject, got %d", len(allowRaw))
	}
}
