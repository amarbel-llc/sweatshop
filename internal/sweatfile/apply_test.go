package sweatfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyGitExcludes(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git", "info")
	os.MkdirAll(gitDir, 0o755)
	excludePath := filepath.Join(gitDir, "exclude")

	err := applyGitExcludes(excludePath, []string{".claude/", ".direnv/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(excludePath)
	if string(data) != ".claude/\n.direnv/\n" {
		t.Errorf("exclude content: got %q", string(data))
	}
}

func TestApplyFilesSymlink(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "source-envrc")
	os.WriteFile(srcFile, []byte("use flake ."), 0o644)

	worktreeDir := filepath.Join(dir, "worktree")
	os.MkdirAll(worktreeDir, 0o755)

	files := map[string]FileEntry{
		"envrc": {Source: srcFile},
	}
	err := ApplyFiles(worktreeDir, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dest := filepath.Join(worktreeDir, ".envrc")
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if target != srcFile {
		t.Errorf("symlink target: got %q", target)
	}
}

func TestApplyFilesContent(t *testing.T) {
	dir := t.TempDir()
	worktreeDir := filepath.Join(dir, "worktree")
	os.MkdirAll(worktreeDir, 0o755)

	files := map[string]FileEntry{
		"tool-versions": {Content: "golang 1.23.0\n"},
	}
	err := ApplyFiles(worktreeDir, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(worktreeDir, ".tool-versions"))
	if string(data) != "golang 1.23.0\n" {
		t.Errorf("content: got %q", string(data))
	}
}

func TestApplyFilesNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	worktreeDir := filepath.Join(dir, "worktree")
	os.MkdirAll(worktreeDir, 0o755)
	os.WriteFile(filepath.Join(worktreeDir, ".envrc"), []byte("existing"), 0o644)

	files := map[string]FileEntry{
		"envrc": {Content: "new content"},
	}
	ApplyFiles(worktreeDir, files)
	data, _ := os.ReadFile(filepath.Join(worktreeDir, ".envrc"))
	if string(data) != "existing" {
		t.Errorf("expected existing content preserved, got %q", string(data))
	}
}

func TestApplyFilesNestedPath(t *testing.T) {
	dir := t.TempDir()
	worktreeDir := filepath.Join(dir, "worktree")
	os.MkdirAll(worktreeDir, 0o755)

	files := map[string]FileEntry{
		"claude/settings.local.json": {Content: `{"permissions":{}}`},
	}
	err := ApplyFiles(worktreeDir, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(worktreeDir, ".claude", "settings.local.json"))
	if string(data) != `{"permissions":{}}` {
		t.Errorf("content: got %q", string(data))
	}
}

func TestApplyEnv(t *testing.T) {
	dir := t.TempDir()
	err := ApplyEnv(dir, map[string]string{"EDITOR": "nvim", "PAGER": "less"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".sweatshop-env"))
	content := string(data)
	if !strings.Contains(content, "EDITOR=nvim") || !strings.Contains(content, "PAGER=less") {
		t.Errorf("env content: got %q", content)
	}
}

func TestApplyEnvEmpty(t *testing.T) {
	dir := t.TempDir()
	err := ApplyEnv(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, ".sweatshop-env"))
	if err == nil {
		t.Error("expected no .sweatshop-env for empty env")
	}
}

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
