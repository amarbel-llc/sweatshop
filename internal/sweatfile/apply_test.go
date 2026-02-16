package sweatfile

import (
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
