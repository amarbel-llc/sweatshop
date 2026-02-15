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
	os.MkdirAll(filepath.Join(tmpDir, "eng", "repos", "myrepo"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "eng/worktrees/myrepo/") {
		t.Errorf("expected repo listing, got %q", output)
	}
	if !strings.Contains(output, "new worktree") {
		t.Errorf("expected 'new worktree' description, got %q", output)
	}
}

func TestLocalListsExistingWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "eng", "repos", "myrepo"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "eng", "worktrees", "myrepo", "feature-x"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "eng/worktrees/myrepo/feature-x") {
		t.Errorf("expected existing worktree, got %q", output)
	}
	if !strings.Contains(output, "existing worktree") {
		t.Errorf("expected 'existing worktree' description, got %q", output)
	}
}

func TestLocalHandlesMultipleEngAreas(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "eng", "repos", "repo-a"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "eng2", "repos", "repo-b"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "eng/worktrees/repo-a/") {
		t.Errorf("expected repo-a, got %q", output)
	}
	if !strings.Contains(output, "eng2/worktrees/repo-b/") {
		t.Errorf("expected repo-b, got %q", output)
	}
}

func TestLocalOutputIsTabSeparated(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "eng", "repos", "myrepo"), 0o755)

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

func TestLocalHandlesNoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}
