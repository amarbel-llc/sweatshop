package worktree

import (
	"os"
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

func TestResolvePathConvention(t *testing.T) {
	home := t.TempDir()
	rp, err := ResolvePath(home, "eng/worktrees/myrepo/feature-x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rp.Convention {
		t.Error("expected Convention=true")
	}
	if rp.Branch != "feature-x" {
		t.Errorf("expected branch feature-x, got %q", rp.Branch)
	}
	if rp.SessionKey != "eng/myrepo/feature-x" {
		t.Errorf("expected session key eng/myrepo/feature-x, got %q", rp.SessionKey)
	}
	if rp.AbsPath != home+"/eng/worktrees/myrepo/feature-x" {
		t.Errorf("unexpected AbsPath: %q", rp.AbsPath)
	}
	if rp.EngAreaDir != home+"/eng" {
		t.Errorf("expected EngAreaDir %q, got %q", home+"/eng", rp.EngAreaDir)
	}
}

func TestResolvePathConventionWithRepoFlag(t *testing.T) {
	home := t.TempDir()
	rp, err := ResolvePath(home, "eng/worktrees/myrepo/feature-x", "/custom/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.RepoPath != "/custom/repo" {
		t.Errorf("expected RepoPath /custom/repo, got %q", rp.RepoPath)
	}
}

func TestResolvePathArbitraryWithRepo(t *testing.T) {
	home := t.TempDir()
	rp, err := ResolvePath(home, "/tmp/my-worktree", "/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.Convention {
		t.Error("expected Convention=false")
	}
	if rp.AbsPath != "/tmp/my-worktree" {
		t.Errorf("expected AbsPath /tmp/my-worktree, got %q", rp.AbsPath)
	}
	if rp.RepoPath != "/some/repo" {
		t.Errorf("expected RepoPath /some/repo, got %q", rp.RepoPath)
	}
	if rp.Branch != "my-worktree" {
		t.Errorf("expected branch my-worktree, got %q", rp.Branch)
	}
	if rp.SessionKey != "/tmp/my-worktree" {
		t.Errorf("expected session key /tmp/my-worktree, got %q", rp.SessionKey)
	}
}

func TestResolvePathArbitraryUnderHomeStripsPrefix(t *testing.T) {
	home := t.TempDir()
	absPath := home + "/projects/my-worktree"
	rp, err := ResolvePath(home, absPath, "/some/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp.SessionKey != "projects/my-worktree" {
		t.Errorf("expected session key projects/my-worktree, got %q", rp.SessionKey)
	}
}

func TestResolvePathArbitraryWithoutRepoFails(t *testing.T) {
	home := t.TempDir()
	_, err := ResolvePath(home, "/tmp/new-worktree", "")
	if err == nil {
		t.Error("expected error for non-convention path without --repo")
	}
}

func TestFindEngAreaDirPositive(t *testing.T) {
	home := t.TempDir()
	engDir := home + "/eng"
	if err := os.MkdirAll(engDir+"/worktrees/repo/branch", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(engDir+"/sweatfile", []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := findEngAreaDir(engDir+"/worktrees/repo/branch", home)
	if got != engDir {
		t.Errorf("expected %q, got %q", engDir, got)
	}
}

func TestFindEngAreaDirNegative(t *testing.T) {
	home := t.TempDir()
	got := findEngAreaDir("/tmp/random/path", home)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
