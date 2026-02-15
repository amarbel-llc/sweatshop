package worktree

import (
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

func TestParsePathInvalid(t *testing.T) {
	_, err := ParsePath("eng/repos/myrepo")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestApplyRcmOverlayCopiesAsDotfiles(t *testing.T) {
	tmpDir := t.TempDir()

	rcmDir := filepath.Join(tmpDir, "eng", "rcm-worktrees", "config", "git")
	os.MkdirAll(rcmDir, 0o755)
	os.WriteFile(filepath.Join(rcmDir, "ignore"), []byte("some-config"), 0o644)

	worktreeDir := filepath.Join(tmpDir, "eng", "worktrees", "myrepo", "feature-x")
	os.MkdirAll(worktreeDir, 0o755)

	// Override HOME for the test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := ApplyRcmOverlay("eng", worktreeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest := filepath.Join(worktreeDir, ".config", "git", "ignore")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected dotfile to exist: %v", err)
	}
	if string(data) != "some-config" {
		t.Errorf("expected some-config, got %q", string(data))
	}
}

func TestApplyRcmOverlayDoesNotOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	rcmDir := filepath.Join(tmpDir, "eng", "rcm-worktrees")
	os.MkdirAll(rcmDir, 0o755)
	os.WriteFile(filepath.Join(rcmDir, "gitignore"), []byte("overlay-content"), 0o644)

	worktreeDir := filepath.Join(tmpDir, "eng", "worktrees", "myrepo", "feature-x")
	os.MkdirAll(worktreeDir, 0o755)
	os.WriteFile(filepath.Join(worktreeDir, ".gitignore"), []byte("existing-content"), 0o644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	ApplyRcmOverlay("eng", worktreeDir)

	data, _ := os.ReadFile(filepath.Join(worktreeDir, ".gitignore"))
	if string(data) != "existing-content" {
		t.Errorf("expected existing-content, got %q", string(data))
	}
}

func TestApplyRcmOverlaySkipsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	worktreeDir := filepath.Join(tmpDir, "eng", "worktrees", "myrepo", "feature-x")
	os.MkdirAll(worktreeDir, 0o755)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	err := ApplyRcmOverlay("eng", worktreeDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
