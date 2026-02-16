package sweatfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMinimal(t *testing.T) {
	input := `
git_excludes = [".claude/"]
setup = ["direnv allow"]

[env]
EDITOR = "nvim"

[files.envrc]
source = "~/eng/rcm-worktrees/envrc"
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.GitExcludes) != 1 || sf.GitExcludes[0] != ".claude/" {
		t.Errorf("git_excludes: got %v", sf.GitExcludes)
	}
	if sf.Env["EDITOR"] != "nvim" {
		t.Errorf("env EDITOR: got %q", sf.Env["EDITOR"])
	}
	if sf.Files["envrc"].Source != "~/eng/rcm-worktrees/envrc" {
		t.Errorf("files.envrc.source: got %q", sf.Files["envrc"].Source)
	}
	if len(sf.Setup) != 1 || sf.Setup[0] != "direnv allow" {
		t.Errorf("setup: got %v", sf.Setup)
	}
}

func TestParseFileContent(t *testing.T) {
	input := `
[files.envrc]
content = "use flake ."
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Files["envrc"].Content != "use flake ." {
		t.Errorf("files.envrc.content: got %q", sf.Files["envrc"].Content)
	}
}

func TestParseEmpty(t *testing.T) {
	sf, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.GitExcludes != nil {
		t.Errorf("expected nil git_excludes, got %v", sf.GitExcludes)
	}
}

func TestLoadFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweatfile")
	os.WriteFile(path, []byte(`git_excludes = [".direnv/"]`), 0o644)

	sf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.GitExcludes) != 1 || sf.GitExcludes[0] != ".direnv/" {
		t.Errorf("git_excludes: got %v", sf.GitExcludes)
	}
}

func TestLoadMissing(t *testing.T) {
	sf, err := Load("/nonexistent/sweatfile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.GitExcludes != nil {
		t.Errorf("expected nil git_excludes, got %v", sf.GitExcludes)
	}
}

func TestMergeConcatenatesArrays(t *testing.T) {
	base := Sweatfile{
		GitExcludes: []string{".claude/"},
		Setup:       []string{"direnv allow"},
	}
	repo := Sweatfile{
		GitExcludes: []string{".direnv/"},
		Setup:       []string{"go mod download"},
	}
	merged := Merge(base, repo)
	if len(merged.GitExcludes) != 2 {
		t.Fatalf("expected 2 git_excludes, got %v", merged.GitExcludes)
	}
	if merged.GitExcludes[0] != ".claude/" || merged.GitExcludes[1] != ".direnv/" {
		t.Errorf("git_excludes: got %v", merged.GitExcludes)
	}
	if len(merged.Setup) != 2 {
		t.Fatalf("expected 2 setup, got %v", merged.Setup)
	}
}

func TestMergeClearSentinel(t *testing.T) {
	base := Sweatfile{
		GitExcludes: []string{".claude/"},
		Setup:       []string{"direnv allow"},
	}
	repo := Sweatfile{
		GitExcludes: []string{},
		Setup:       []string{},
	}
	merged := Merge(base, repo)
	if len(merged.GitExcludes) != 0 {
		t.Errorf("expected cleared git_excludes, got %v", merged.GitExcludes)
	}
	if len(merged.Setup) != 0 {
		t.Errorf("expected cleared setup, got %v", merged.Setup)
	}
}

func TestMergeEnvOverride(t *testing.T) {
	base := Sweatfile{Env: map[string]string{"EDITOR": "vim", "PAGER": "less"}}
	repo := Sweatfile{Env: map[string]string{"EDITOR": "nvim"}}
	merged := Merge(base, repo)
	if merged.Env["EDITOR"] != "nvim" {
		t.Errorf("expected nvim, got %q", merged.Env["EDITOR"])
	}
	if merged.Env["PAGER"] != "less" {
		t.Errorf("expected less, got %q", merged.Env["PAGER"])
	}
}

func TestMergeFilesOverride(t *testing.T) {
	base := Sweatfile{
		Files: map[string]FileEntry{
			"envrc":     {Source: "~/eng/rcm-worktrees/envrc"},
			"gitconfig": {Source: "~/eng/rcm-worktrees/gitconfig"},
		},
	}
	repo := Sweatfile{
		Files: map[string]FileEntry{
			"envrc": {Content: "use flake ."},
		},
	}
	merged := Merge(base, repo)
	if merged.Files["envrc"].Content != "use flake ." {
		t.Errorf("expected inline content, got %+v", merged.Files["envrc"])
	}
	if merged.Files["gitconfig"].Source != "~/eng/rcm-worktrees/gitconfig" {
		t.Errorf("expected inherited gitconfig, got %+v", merged.Files["gitconfig"])
	}
}

func TestMergeBaseOnly(t *testing.T) {
	base := Sweatfile{GitExcludes: []string{".claude/"}}
	merged := Merge(base, Sweatfile{})
	if len(merged.GitExcludes) != 1 || merged.GitExcludes[0] != ".claude/" {
		t.Errorf("expected inherited git_excludes, got %v", merged.GitExcludes)
	}
}
