package worktree

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	if err := sweatfile.Apply(worktreePath, sf); err != nil {
		return err
	}

	return injectWorktreePerms(worktreePath)
}

func injectWorktreePerms(worktreePath string) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")

	// Load existing settings
	var doc map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &doc)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if doc == nil {
		doc = make(map[string]any)
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	if permsMap == nil {
		permsMap = make(map[string]any)
	}

	var existing []string
	if allowRaw, ok := permsMap["allow"].([]any); ok {
		for _, v := range allowRaw {
			if s, ok := v.(string); ok {
				existing = append(existing, s)
			}
		}
	}

	rules := appendUnique(existing,
		"Read("+worktreePath+"/*)",
		"Write("+worktreePath+"/*)",
		"Edit("+worktreePath+"/*)",
	)

	permsMap["allow"] = rules
	doc["permissions"] = permsMap

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}

func IsWorktree(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func appendUnique(existing []string, rules ...string) []string {
	set := make(map[string]bool, len(existing))
	for _, r := range existing {
		set[r] = true
	}
	result := append([]string{}, existing...)
	for _, r := range rules {
		if !set[r] {
			result = append(result, r)
		}
	}
	return result
}
