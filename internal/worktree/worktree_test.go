package worktree

import (
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
