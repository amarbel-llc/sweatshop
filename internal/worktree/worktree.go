package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/sweatfile"
)

type Target struct {
	Host string
	Path string
}

func ParseTarget(target string) Target {
	if idx := strings.IndexByte(target, ':'); idx >= 0 {
		return Target{
			Host: target[:idx],
			Path: target[idx+1:],
		}
	}
	return Target{Path: target}
}

type PathComponents struct {
	EngArea  string
	Repo     string
	Worktree string
}

func ParsePath(path string) (PathComponents, error) {
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[1] != "worktrees" {
		return PathComponents{}, fmt.Errorf("invalid worktree path: %s (expected <eng_area>/worktrees/<repo>/<branch>)", path)
	}
	return PathComponents{
		EngArea:  parts[0],
		Repo:     parts[2],
		Worktree: parts[3],
	}, nil
}

func (c PathComponents) ShopKey() string {
	return c.EngArea + "/" + c.Repo + "/" + c.Worktree
}

func RepoPath(home string, comp PathComponents) string {
	return filepath.Join(home, comp.EngArea, "repos", comp.Repo)
}

func WorktreePath(home string, sweatshopPath string) string {
	return filepath.Join(home, sweatshopPath)
}

func Create(engArea, repoPath, worktreePath string) error {
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		return fmt.Errorf("creating worktree directory: %w", err)
	}
	if err := git.RunPassthrough(repoPath, "worktree", "add", worktreePath); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	engAreaDir := filepath.Join(home, engArea)
	sf, err := sweatfile.LoadMerged(engAreaDir, repoPath)
	if err != nil {
		return fmt.Errorf("loading sweatfile: %w", err)
	}
	return sweatfile.Apply(worktreePath, sf)
}

func IsWorktree(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}
